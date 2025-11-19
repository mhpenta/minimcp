package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mhpenta/minimcp/tools"
)

// JSON-RPC 2.0 message structures
// See: https://www.jsonrpc.org/specification

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"` // Can be string, number, or null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no ID, no response expected)
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP-specific method names
const (
	MethodInitialize = "initialize"
	MethodToolsList  = "tools/list"
	MethodToolsCall  = "tools/call"
)

// InitializeParams represents MCP initialize request parameters
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities,omitempty"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo represents information about the MCP client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents MCP initialize response
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes what the server supports
type ServerCapabilities struct {
	Tools map[string]interface{} `json:"tools,omitempty"`
}

// ServerInfo represents information about the MCP server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResult represents the response for tools/list
type ToolsListResult struct {
	Tools []ToolDescription `json:"tools"`
}

// ToolDescription represents a tool in MCP format
type ToolDescription struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolsCallParams represents parameters for tools/call
type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolsCallResult represents the response for tools/call
type ToolsCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// JSONRPCHandler handles JSON-RPC 2.0 messages for MCP protocol
type JSONRPCHandler struct {
	server *Server
}

// NewJSONRPCHandler creates a new JSON-RPC handler
func NewJSONRPCHandler(server *Server) *JSONRPCHandler {
	return &JSONRPCHandler{
		server: server,
	}
}

// HandleMessage processes a JSON-RPC message and returns a response
// Returns nil if the message is a notification (no response expected)
func (h *JSONRPCHandler) HandleMessage(ctx context.Context, data []byte) (*JSONRPCResponse, error) {
	// First, try to parse as a request (has ID)
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    ParseError,
				Message: "Parse error",
				Data:    err.Error(),
			},
		}, nil
	}

	// Check if it's a notification (no ID field)
	if req.ID == nil {
		// It's a notification, no response needed
		h.server.logger.Info("received notification", "method", req.Method)
		return nil, nil
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    InvalidRequest,
				Message: "Invalid JSON-RPC version",
			},
		}, nil
	}

	// Route to appropriate method handler
	var result interface{}
	var rpcErr *RPCError

	switch req.Method {
	case MethodInitialize:
		result, rpcErr = h.handleInitialize(ctx, req.Params)
	case MethodToolsList:
		result, rpcErr = h.handleToolsList(ctx, req.Params)
	case MethodToolsCall:
		result, rpcErr = h.handleToolsCall(ctx, req.Params)
	default:
		rpcErr = &RPCError{
			Code:    MethodNotFound,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	}, nil
}

// handleInitialize processes the initialize request
func (h *JSONRPCHandler) handleInitialize(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var initParams InitializeParams
	if params != nil {
		if err := json.Unmarshal(params, &initParams); err != nil {
			return nil, &RPCError{
				Code:    InvalidParams,
				Message: "Invalid initialize parameters",
				Data:    err.Error(),
			}
		}
	}

	h.server.logger.Info("MCP client connected",
		"client", initParams.ClientInfo.Name,
		"version", initParams.ClientInfo.Version)

	return InitializeResult{
		ProtocolVersion: "2024-11-05", // MCP protocol version
		Capabilities: ServerCapabilities{
			Tools: map[string]interface{}{
				"listChanged": true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    h.server.name,
			Version: h.server.version,
		},
	}, nil
}

// handleToolsList processes the tools/list request
func (h *JSONRPCHandler) handleToolsList(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	toolList := make([]ToolDescription, 0, len(h.server.tools))
	for _, tool := range h.server.tools {
		spec := tool.Spec()

		// Normalize the input schema to ensure "required" is always an array, not null
		// This is required by JSON Schema spec and some MCP clients reject null values
		inputSchema := normalizeJSONSchema(spec.Parameters)

		toolList = append(toolList, ToolDescription{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: inputSchema,
		})
	}

	return ToolsListResult{
		Tools: toolList,
	}, nil
}

// normalizeJSONSchema ensures the schema conforms to JSON Schema spec
// Specifically, it ensures "required" is an empty array instead of null
func normalizeJSONSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return schema
	}

	// Marshal and unmarshal to get a deep copy, then fix the required field
	data, err := json.Marshal(schema)
	if err != nil {
		return schema
	}

	var normalized map[string]interface{}
	if err := json.Unmarshal(data, &normalized); err != nil {
		return schema
	}

	// Fix the "required" field if it's null or doesn't exist
	if required, exists := normalized["required"]; !exists || required == nil {
		normalized["required"] = []string{}
	}

	return normalized
}

// handleToolsCall processes the tools/call request
func (h *JSONRPCHandler) handleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var callParams ToolsCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &RPCError{
			Code:    InvalidParams,
			Message: "Invalid tools/call parameters",
			Data:    err.Error(),
		}
	}

	h.server.logger.Info("executing tool via JSON-RPC", "tool", callParams.Name)

	// Find the tool
	var targetTool tools.Tool
	for _, tool := range h.server.tools {
		if tool.Spec().Name == callParams.Name {
			targetTool = tool
			break
		}
	}

	if targetTool == nil {
		return nil, &RPCError{
			Code:    InvalidParams,
			Message: fmt.Sprintf("Tool not found: %s", callParams.Name),
		}
	}

	// Execute the tool
	result, err := targetTool.Execute(ctx, callParams.Arguments)
	if err != nil {
		// Check if it's a specific tool error
		var toolErr *tools.Error
		if errors.As(err, &toolErr) {
			// If the error code is within the reserved JSON-RPC error range (-32768 to -32000),
			// we treat it as a protocol-level error and return it directly.
			// This allows tools to return InvalidParams, InternalError, or other standard codes.
			if toolErr.Code >= -32768 && toolErr.Code <= -32000 {
				return nil, &RPCError{
					Code:    toolErr.Code,
					Message: toolErr.Message,
					Data:    toolErr.Data,
				}
			}
		}

		h.server.logger.Error("MCP JSON-RPC tool execution failed",
			"tool", callParams.Name,
			"error", err.Error(),
			"errorType", fmt.Sprintf("%T", err),
			"arguments", string(callParams.Arguments),
			"context", "mcp_jsonrpc_handler")

		return ToolsCallResult{
			Content: []ContentBlock{
				{
					Type: "text",
					Text: fmt.Sprintf("Error executing tool: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Convert tool result to MCP response format
	var text string
	if result.Error != nil {
		text = *result.Error
	} else if result.Output != nil {
		text = tools.MarshalOutput(h.server.logger, result.Output)
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

	return ToolsCallResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: false,
	}, nil
}
