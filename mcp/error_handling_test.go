package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mhpenta/minimcp/mcp"
	"github.com/mhpenta/minimcp/tools"
)

type TestInput struct {
	Val int `json:"val"`
}

func TestErrorHandling_InvalidParams(t *testing.T) {
	// Create a tool
	tool := tools.NewTool("test_tool", "desc", func(ctx context.Context, input TestInput) (string, error) {
		return "ok", nil
	})

	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "test",
		Version: "1.0",
		Tools:   []tools.Tool{tool},
	})

	handler := mcp.NewJSONRPCHandler(server)

	// Case 1: Invalid Params (Type mismatch)
	// "val" should be int, passing string
	// This should trigger safeunmarshal failure -> tools.InvalidParamsError -> RPCError
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "test_tool", "arguments": {"val": "not_an_int"}}`),
	}
	reqBytes, _ := json.Marshal(req)

	resp, err := handler.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	// We expect an Error in the response, because InvalidParams is a protocol error
	if resp.Error == nil {
		t.Fatal("Expected error in response, got nil")
	}
	if resp.Error.Code != -32602 { // InvalidParams
		t.Errorf("Expected error code -32602, got %d. Message: %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestErrorHandling_ToolExecutionError(t *testing.T) {
	// Create a tool that always fails
	tool := tools.NewTool("fail_tool", "desc", func(ctx context.Context, input TestInput) (string, error) {
		return "", tools.NewError(1, "something went wrong")
	})

	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "test",
		Version: "1.0",
		Tools:   []tools.Tool{tool},
	})

	handler := mcp.NewJSONRPCHandler(server)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "fail_tool", "arguments": {"val": 1}}`),
	}
	reqBytes, _ := json.Marshal(req)

	resp, err := handler.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	// For general tool errors, we expect a SUCCESSFUL JSON-RPC response (Error is nil)
	// but the Result should indicate IsError=true
	if resp.Error != nil {
		t.Fatalf("Expected nil RPC Error, got: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var toolResult mcp.ToolsCallResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !toolResult.IsError {
		t.Error("Expected IsError=true in tool result")
	}
}

func TestErrorHandling_ReservedErrorCode(t *testing.T) {
	// Create a tool that returns a reserved error code (e.g., -32001)
	// This should be treated as a protocol error, not a tool execution error
	tool := tools.NewTool("reserved_error_tool", "desc", func(ctx context.Context, input TestInput) (string, error) {
		return "", tools.NewError(-32001, "custom protocol error")
	})

	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "test",
		Version: "1.0",
		Tools:   []tools.Tool{tool},
	})

	handler := mcp.NewJSONRPCHandler(server)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "reserved_error_tool", "arguments": {"val": 1}}`),
	}
	reqBytes, _ := json.Marshal(req)

	resp, err := handler.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	// We expect a JSON-RPC Error response because -32001 is in the reserved range
	if resp.Error == nil {
		t.Fatal("Expected error in response, got nil")
	}
	if resp.Error.Code != -32001 {
		t.Errorf("Expected error code -32001, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "custom protocol error" {
		t.Errorf("Expected message 'custom protocol error', got '%s'", resp.Error.Message)
	}
}
