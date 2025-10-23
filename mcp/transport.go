package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mhpenta/minimcp/tools"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// AuthHeaderType defines the type of authentication header to use
type AuthHeaderType string

const (
	AuthHeaderBearer AuthHeaderType = "bearer"  // Authorization: Bearer <token>
	AuthHeaderAPIKey AuthHeaderType = "api-key" // X-API-Key: <token>
)

// HTTPTransport provides HTTP-based MCP server
type HTTPTransport struct {
	server         *Server
	router         *http.ServeMux
	logger         *slog.Logger
	apiKey         APIKeyValidator
	jsonrpcHandler *JSONRPCHandler
	authHeaderType AuthHeaderType // Configurable auth header type
}

// NewHTTPTransport creates a new HTTP transport for the MCP server
// By default, uses Authorization: Bearer authentication (recommended for MCP/Claude Code)
func NewHTTPTransport(
	server *Server,
	logger *slog.Logger,
	apiKeyValidator APIKeyValidator) *HTTPTransport {

	router := http.NewServeMux()
	transport := &HTTPTransport{
		server:         server,
		router:         router,
		logger:         logger,
		apiKey:         apiKeyValidator,
		jsonrpcHandler: NewJSONRPCHandler(server),
		authHeaderType: AuthHeaderBearer, // Default to Bearer auth
	}

	// Register MCP JSON-RPC endpoint (Claude Code compatible)
	router.HandleFunc("/mcp", transport.authMiddleware(transport.handleMCP))

	// Register REST endpoints (for simple HTTP clients)
	router.HandleFunc("/mcp/tools/list", transport.authMiddleware(transport.handleListTools))
	router.HandleFunc("/mcp/tools/call", transport.authMiddleware(transport.handleCallTool))
	router.HandleFunc("/mcp/health", transport.handleHealth)

	return transport
}

// WithAuthHeaderType sets the authentication header type (bearer or api-key)
func (t *HTTPTransport) WithAuthHeaderType(headerType AuthHeaderType) *HTTPTransport {
	t.authHeaderType = headerType
	return t
}

// authMiddleware validates authentication based on configured header type
func (t *HTTPTransport) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var providedKey string

		// Extract key based on configured auth header type
		switch t.authHeaderType {
		case AuthHeaderBearer:
			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				providedKey = authHeader[7:]
			}
		case AuthHeaderAPIKey:
			// Extract from X-API-Key header
			providedKey = r.Header.Get("X-API-Key")
		default:
			// Fallback to Bearer
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				providedKey = authHeader[7:]
			}
		}

		// Validate the key
		if !t.apiKey.Validate(r.Context(), providedKey) {
			t.logger.Warn("unauthorized MCP request",
				"auth_type", t.authHeaderType,
				"has_key", providedKey != "",
				"header", r.Header)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handleMCP handles MCP JSON-RPC protocol requests (Claude Code compatible)
func (t *HTTPTransport) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests for JSON-RPC
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed, use POST for JSON-RPC requests", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.logger.Error("failed to read request body", "error", err)
		http.Error(w, fmt.Sprintf("failed to read request: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Check if it's a batch request (array of requests)
	var isBatch bool
	var requests []json.RawMessage

	// Try to parse as array first
	if err := json.Unmarshal(body, &requests); err == nil && len(requests) > 0 {
		isBatch = true
	} else {
		// Single request
		requests = []json.RawMessage{body}
		isBatch = false
	}

	// Process each request
	responses := make([]*JSONRPCResponse, 0, len(requests))
	for _, reqData := range requests {
		resp, err := t.jsonrpcHandler.HandleMessage(r.Context(), reqData)
		if err != nil {
			t.logger.Error("error handling JSON-RPC message", "error", err)
			responses = append(responses, &JSONRPCResponse{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    InternalError,
					Message: "Internal server error",
					Data:    err.Error(),
				},
			})
		} else if resp != nil {
			// Only add response if it's not a notification
			responses = append(responses, resp)
		}
	}

	// Don't send a response for notifications (empty responses)
	if len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if isBatch {
		json.NewEncoder(w).Encode(responses)
	} else if len(responses) > 0 {
		json.NewEncoder(w).Encode(responses[0])
	}
}

// handleHealth returns server health status
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// handleListTools returns the list of available tools
func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	toolList := make([]map[string]interface{}, 0, len(t.server.tools))
	for _, tool := range t.server.tools {
		spec := tool.Spec()
		toolList = append(toolList, map[string]interface{}{
			"name":        spec.Name,
			"description": spec.Description,
			"inputSchema": spec.Parameters,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": toolList,
	})
}

// CallToolRequest represents an MCP tool call request
type CallToolRequest struct {
	Name   string          `json:"name"`
	Params json.RawMessage `json:"arguments"`
}

// CallToolResponse represents an MCP tool call response
type CallToolResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in the response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// handleCallTool executes a tool and returns the result
func (t *HTTPTransport) handleCallTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CallToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.logger.Error("failed to decode request", "error", err)
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	t.logger.Info("executing tool", "tool", req.Name)

	// Find the tool
	var targetTool tools.Tool
	for _, tool := range t.server.tools {
		if tool.Spec().Name == req.Name {
			targetTool = tool
			break
		}
	}

	if targetTool == nil {
		t.logger.Warn("tool not found", "tool", req.Name)
		http.Error(w, fmt.Sprintf("tool not found: %s", req.Name), http.StatusNotFound)
		return
	}

	// Execute the tool with context
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	result, err := targetTool.Execute(ctx, req.Params)
	if err != nil {
		t.logger.Error("MCP tool execution failed",
			"tool", req.Name,
			"error", err.Error(),
			"errorType", fmt.Sprintf("%T", err),
			"arguments", string(req.Params),
			"context", "mcp_http_transport")
		response := CallToolResponse{
			Content: []ContentBlock{
				{
					Type: "text",
					Text: fmt.Sprintf("Error executing tool: %v", err),
				},
			},
			IsError: true,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // MCP protocol uses 200 even for tool errors
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert tool result to MCP response format
	var text string
	if result.Error != nil {
		text = *result.Error
	} else if result.Output != nil {
		text = tools.MarshalOutput(t.logger, result.Output)
	} else if result.System != nil {
		text = *result.System
	} else {
		// Fallback to JSON marshaling the entire result
		resultBytes, err := json.Marshal(result)
		if err != nil {
			text = "Error serializing result"
		} else {
			text = string(resultBytes)
		}
	}

	response := CallToolResponse{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ServeHTTP implements http.Handler
func (t *HTTPTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.router.ServeHTTP(w, r)
}

// Start starts the HTTP server on the specified port with graceful shutdown support
func (t *HTTPTransport) Start(ctx context.Context, port string) error {
	addr := ":" + port
	t.logger.Info("starting MCP HTTP server", "addr", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      t,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to capture server errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		t.logger.Info("HTTP server listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		t.logger.Info("shutting down MCP server gracefully...")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.logger.Error("error during server shutdown", "error", err)
			return fmt.Errorf("server shutdown error: %w", err)
		}

		t.logger.Info("MCP server stopped gracefully")
		return nil
	}
}
