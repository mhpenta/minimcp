package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhpenta/minimcp/tools"
)

// Mock API key validator for testing
type mockAPIKeyValidator struct {
	validKeys map[string]bool
}

func (m *mockAPIKeyValidator) Validate(ctx context.Context, apiKey string) bool {
	return m.validKeys[apiKey]
}

func newMockValidator(validKeys ...string) *mockAPIKeyValidator {
	validator := &mockAPIKeyValidator{
		validKeys: make(map[string]bool),
	}
	for _, key := range validKeys {
		validator.validKeys[key] = true
	}
	return validator
}

func TestHTTPTransport_Health(t *testing.T) {
	logger := slog.Default()
	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/mcp/health", nil)
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", response["status"])
	}

	if response["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", response["version"])
	}
}

func TestHTTPTransport_AuthMiddleware_Bearer(t *testing.T) {
	logger := slog.Default()
	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("valid-token")
	transport := NewHTTPTransport(server, logger, validator)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "valid bearer token",
			authHeader:     "Bearer valid-token",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid bearer token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "malformed bearer header",
			authHeader:     "Bearer",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp/tools/list", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			transport.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHTTPTransport_AuthMiddleware_APIKey(t *testing.T) {
	logger := slog.Default()
	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("valid-api-key")
	transport := NewHTTPTransport(server, logger, validator).WithAuthHeaderType(AuthHeaderAPIKey)

	tests := []struct {
		name           string
		apiKeyHeader   string
		expectedStatus int
	}{
		{
			name:           "valid api key",
			apiKeyHeader:   "valid-api-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid api key",
			apiKeyHeader:   "invalid-key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing api key",
			apiKeyHeader:   "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp/tools/list", nil)
			if tt.apiKeyHeader != "" {
				req.Header.Set("X-API-Key", tt.apiKeyHeader)
			}
			w := httptest.NewRecorder()

			transport.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHTTPTransport_ListTools(t *testing.T) {
	logger := slog.Default()

	tool1 := &mockTool{
		name:        "echo",
		description: "Echoes input",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{tool1},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/mcp/tools/list", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	toolsInterface, ok := response["tools"].([]interface{})
	if !ok {
		t.Fatal("expected tools array in response")
	}

	if len(toolsInterface) != 1 {
		t.Errorf("expected 1 tool, got %d", len(toolsInterface))
	}

	tool := toolsInterface[0].(map[string]interface{})
	if tool["name"] != "echo" {
		t.Errorf("expected tool name 'echo', got %v", tool["name"])
	}
}

func TestHTTPTransport_CallTool(t *testing.T) {
	logger := slog.Default()

	echoTool := &mockTool{
		name:        "echo",
		description: "Echoes input",
		parameters:  map[string]interface{}{"type": "object"},
		result: &tools.ToolResult{
			Output: "Hello, World!",
		},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{echoTool},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	reqBody := CallToolRequest{
		Name:   "echo",
		Params: json.RawMessage(`{"message":"Hello, World!"}`),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response CallToolResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.IsError {
		t.Error("expected IsError to be false")
	}

	if len(response.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(response.Content))
	}

	if response.Content[0].Text != "Hello, World!" {
		t.Errorf("expected text 'Hello, World!', got %s", response.Content[0].Text)
	}
}

func TestHTTPTransport_CallTool_NotFound(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	reqBody := CallToolRequest{
		Name:   "nonexistent",
		Params: json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHTTPTransport_CallTool_ExecutionError(t *testing.T) {
	logger := slog.Default()

	errorTool := &mockTool{
		name:        "error_tool",
		description: "A tool that errors",
		parameters:  map[string]interface{}{"type": "object"},
		err:         errors.New("execution failed"),
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{errorTool},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	reqBody := CallToolRequest{
		Name:   "error_tool",
		Params: json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response CallToolResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.IsError {
		t.Error("expected IsError to be true")
	}

	if !strings.Contains(response.Content[0].Text, "Error executing tool") {
		t.Errorf("expected error message, got: %s", response.Content[0].Text)
	}
}

func TestHTTPTransport_CallTool_InvalidJSON(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", strings.NewReader("{invalid json}"))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHTTPTransport_MCP_Initialize(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"clientInfo": {
				"name": "test-client",
				"version": "1.0"
			}
		}`),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Errorf("expected no error, got: %v", response.Error)
	}

	if response.Result == nil {
		t.Fatal("expected result, got nil")
	}

	resultBytes, _ := json.Marshal(response.Result)
	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %s", initResult.ServerInfo.Name)
	}
}

func TestHTTPTransport_MCP_ToolsList(t *testing.T) {
	logger := slog.Default()

	tool1 := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		parameters:  map[string]interface{}{"type": "object"},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{tool1},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var toolsList ToolsListResult
	if err := json.Unmarshal(resultBytes, &toolsList); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(toolsList.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(toolsList.Tools))
	}

	if toolsList.Tools[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %s", toolsList.Tools[0].Name)
	}
}

func TestHTTPTransport_MCP_ToolsCall(t *testing.T) {
	logger := slog.Default()

	echoTool := &mockTool{
		name:        "echo",
		description: "Echoes input",
		parameters:  map[string]interface{}{"type": "object"},
		result: &tools.ToolResult{
			Output: "test output",
		},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{echoTool},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "echo",
			"arguments": {"message": "test"}
		}`),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var callResult ToolsCallResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if callResult.IsError {
		t.Error("expected IsError to be false")
	}

	if callResult.Content[0].Text != "test output" {
		t.Errorf("expected 'test output', got %s", callResult.Content[0].Text)
	}
}

func TestHTTPTransport_MCP_BatchRequest(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	// Create batch request
	batch := []JSONRPCRequest{
		{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: json.RawMessage(`{
				"protocolVersion": "2024-11-05",
				"clientInfo": {"name": "test", "version": "1.0"}
			}`),
		},
		{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/list",
		},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var responses []JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&responses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("expected 2 responses, got %d", len(responses))
	}

	if responses[0].ID != float64(1) {
		t.Errorf("expected first response ID 1, got %v", responses[0].ID)
	}

	if responses[1].ID != float64(2) {
		t.Errorf("expected second response ID 2, got %v", responses[1].ID)
	}
}

func TestHTTPTransport_MCP_Notification(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	// Notification has no ID field
	reqBody := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	// Notifications return 202 Accepted with no body
	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if len(body) > 0 {
		t.Errorf("expected empty body for notification, got: %s", string(body))
	}
}

func TestHTTPTransport_MCP_InvalidMethod(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHTTPTransport_MCP_InvalidJSON(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	validator := newMockValidator("test-key")
	transport := NewHTTPTransport(server, logger, validator)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{invalid json}"))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Error("expected error in response")
	}

	if response.Error.Code != ParseError {
		t.Errorf("expected parse error code, got %d", response.Error.Code)
	}
}
