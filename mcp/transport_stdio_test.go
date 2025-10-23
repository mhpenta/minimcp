package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mhpenta/minimcp/tools"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// Mock tool implementations for testing

type mockTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	result      *tools.ToolResult
	err         error
	executeFn   func(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error)
}

func (m *mockTool) Spec() *tools.ToolSpec {
	return &tools.ToolSpec{
		Name:        m.name,
		Description: m.description,
		Parameters:  m.parameters,
	}
}

func (m *mockTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, params)
	}
	return m.result, m.err
}

func TestStdioTransport_BasicInitialize(t *testing.T) {
	// Create a simple test server with no tools
	logger := slog.Default()
	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{}, // Empty tool list for this test
		Logger:  logger,
	})

	// Create input/output buffers
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-client","version":"1.0"}}}` + "\n")
	output := &bytes.Buffer{}

	// Create transport with test buffers
	transport := NewStdioTransportWithIO(server, logger, input, output)

	// Run transport with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start transport in goroutine
	done := make(chan error, 1)
	go func() {
		done <- transport.Start(ctx)
	}()

	// Wait for processing or timeout
	select {
	case err := <-done:
		if err != nil && err != context.DeadlineExceeded {
			t.Fatalf("transport failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Processing complete, cancel context
		cancel()
	}

	// Parse output
	outputStr := output.String()
	if outputStr == "" {
		t.Fatal("expected response, got empty output")
	}

	// Split by newlines (in case there are multiple responses)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one response line")
	}

	// Parse the first response
	var response JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v\nOutput: %s", err, outputStr)
	}

	// Verify response structure
	if response.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", response.JSONRPC)
	}

	if response.Error != nil {
		t.Errorf("expected no error, got: %v", response.Error)
	}

	// Verify initialize result
	if response.Result == nil {
		t.Fatal("expected result, got nil")
	}

	// Convert result to InitializeResult
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("failed to unmarshal initialize result: %v", err)
	}

	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %s", initResult.ServerInfo.Name)
	}

	if initResult.ServerInfo.Version != "1.0.0" {
		t.Errorf("expected server version '1.0.0', got %s", initResult.ServerInfo.Version)
	}

	if initResult.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version '2024-11-05', got %s", initResult.ProtocolVersion)
	}
}

func TestStdioTransport_ToolsList(t *testing.T) {
	// Create a test server with no tools
	logger := slog.Default()
	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{}, // Empty tool list
		Logger:  logger,
	})

	// Create input/output buffers
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	output := &bytes.Buffer{}

	// Create transport with test buffers
	transport := NewStdioTransportWithIO(server, logger, input, output)

	// Run transport with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start transport in goroutine
	done := make(chan error, 1)
	go func() {
		done <- transport.Start(ctx)
	}()

	// Wait for processing or timeout
	select {
	case err := <-done:
		if err != nil && err != context.DeadlineExceeded {
			t.Fatalf("transport failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Processing complete, cancel context
		cancel()
	}

	// Parse output
	outputStr := output.String()
	if outputStr == "" {
		t.Fatal("expected response, got empty output")
	}

	// Parse the response
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	var response JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v\nOutput: %s", err, outputStr)
	}

	// Verify response structure
	if response.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", response.JSONRPC)
	}

	if response.Error != nil {
		t.Errorf("expected no error, got: %v", response.Error)
	}

	// Verify tools list result
	if response.Result == nil {
		t.Fatal("expected result, got nil")
	}

	// Convert result to ToolsListResult
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var toolsList ToolsListResult
	if err := json.Unmarshal(resultBytes, &toolsList); err != nil {
		t.Fatalf("failed to unmarshal tools list result: %v", err)
	}

	// Should have empty tools array
	if toolsList.Tools == nil {
		t.Error("expected tools array, got nil")
	}

	if len(toolsList.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(toolsList.Tools))
	}
}

func TestStdioTransport_Notification(t *testing.T) {
	// Create a test server
	logger := slog.Default()
	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	// Send a notification (no ID field, no response expected)
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	output := &bytes.Buffer{}

	// Create transport with test buffers
	transport := NewStdioTransportWithIO(server, logger, input, output)

	// Run transport with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start transport in goroutine
	done := make(chan error, 1)
	go func() {
		done <- transport.Start(ctx)
	}()

	// Wait for processing or timeout
	select {
	case err := <-done:
		if err != nil && err != context.DeadlineExceeded {
			t.Fatalf("transport failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Processing complete, cancel context
		cancel()
	}

	// Notifications should produce no output
	outputStr := strings.TrimSpace(output.String())
	if outputStr != "" {
		t.Errorf("expected no output for notification, got: %s", outputStr)
	}
}

func TestStdioTransport_ToolsListWithTools(t *testing.T) {
	logger := slog.Default()

	// Create mock tools
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

	tool2 := &mockTool{
		name:        "add",
		description: "Adds two numbers",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"a": map[string]interface{}{"type": "number"},
				"b": map[string]interface{}{"type": "number"},
			},
		},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{tool1, tool2},
		Logger:  logger,
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var toolsList ToolsListResult
	if err := json.Unmarshal(resultBytes, &toolsList); err != nil {
		t.Fatalf("failed to unmarshal tools list: %v", err)
	}

	if len(toolsList.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(toolsList.Tools))
	}

	if toolsList.Tools[0].Name != "echo" {
		t.Errorf("expected first tool name 'echo', got %s", toolsList.Tools[0].Name)
	}

	if toolsList.Tools[1].Name != "add" {
		t.Errorf("expected second tool name 'add', got %s", toolsList.Tools[1].Name)
	}
}

func TestStdioTransport_ToolsCallSuccess(t *testing.T) {
	logger := slog.Default()

	// Create mock tool that returns output
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

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"Hello, World!"}}}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("expected no error, got: %v", response.Error)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var callResult ToolsCallResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal call result: %v", err)
	}

	if callResult.IsError {
		t.Error("expected IsError to be false")
	}

	if len(callResult.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(callResult.Content))
	}

	if callResult.Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got %s", callResult.Content[0].Type)
	}

	if callResult.Content[0].Text != "Hello, World!" {
		t.Errorf("expected text 'Hello, World!', got %s", callResult.Content[0].Text)
	}
}

func TestStdioTransport_ToolsCallWithError(t *testing.T) {
	logger := slog.Default()

	// Create mock tool that returns an error
	errorMsg := "Something went wrong"
	errorTool := &mockTool{
		name:        "failing_tool",
		description: "A tool that fails",
		parameters:  map[string]interface{}{"type": "object"},
		result: &tools.ToolResult{
			Error: &errorMsg,
		},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{errorTool},
		Logger:  logger,
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"failing_tool","arguments":{}}}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var callResult ToolsCallResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal call result: %v", err)
	}

	if len(callResult.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(callResult.Content))
	}

	if callResult.Content[0].Text != errorMsg {
		t.Errorf("expected text '%s', got %s", errorMsg, callResult.Content[0].Text)
	}
}

func TestStdioTransport_ToolsCallExecutionError(t *testing.T) {
	logger := slog.Default()

	// Create mock tool that fails during execution
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

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"error_tool","arguments":{}}}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var callResult ToolsCallResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal call result: %v", err)
	}

	if !callResult.IsError {
		t.Error("expected IsError to be true")
	}

	if !strings.Contains(callResult.Content[0].Text, "Error executing tool") {
		t.Errorf("expected error message to contain 'Error executing tool', got: %s", callResult.Content[0].Text)
	}
}

func TestStdioTransport_ToolNotFound(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if response.Error.Code != InvalidParams {
		t.Errorf("expected error code %d, got %d", InvalidParams, response.Error.Code)
	}

	if !strings.Contains(response.Error.Message, "Tool not found") {
		t.Errorf("expected error message to contain 'Tool not found', got: %s", response.Error.Message)
	}
}

func TestStdioTransport_InvalidJSON(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	input := bytes.NewBufferString(`{invalid json}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if response.Error.Code != ParseError {
		t.Errorf("expected error code %d, got %d", ParseError, response.Error.Code)
	}
}

func TestStdioTransport_UnknownMethod(t *testing.T) {
	logger := slog.Default()

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{},
		Logger:  logger,
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"unknown/method"}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if response.Error.Code != MethodNotFound {
		t.Errorf("expected error code %d, got %d", MethodNotFound, response.Error.Code)
	}

	if !strings.Contains(response.Error.Message, "Method not found") {
		t.Errorf("expected error message to contain 'Method not found', got: %s", response.Error.Message)
	}
}

func TestStdioTransport_MultipleMessages(t *testing.T) {
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

	// Send multiple messages
	input := bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-client","version":"1.0"}}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{}}}` + "\n",
	)
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(lines))
	}

	// Verify first response is initialize
	var initResponse JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResponse); err != nil {
		t.Fatalf("failed to parse init response: %v", err)
	}
	if initResponse.ID != float64(1) {
		t.Errorf("expected ID 1, got %v", initResponse.ID)
	}

	// Verify second response is tools/list
	var listResponse JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResponse); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if listResponse.ID != float64(2) {
		t.Errorf("expected ID 2, got %v", listResponse.ID)
	}

	// Verify third response is tools/call
	var callResponse JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &callResponse); err != nil {
		t.Fatalf("failed to parse call response: %v", err)
	}
	if callResponse.ID != float64(3) {
		t.Errorf("expected ID 3, got %v", callResponse.ID)
	}
}

func TestStdioTransport_SystemOutput(t *testing.T) {
	logger := slog.Default()

	systemMsg := "System information here"
	systemTool := &mockTool{
		name:        "system_tool",
		description: "Returns system info",
		parameters:  map[string]interface{}{"type": "object"},
		result: &tools.ToolResult{
			System: &systemMsg,
		},
	}

	server := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Tools:   []tools.Tool{systemTool},
		Logger:  logger,
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"system_tool","arguments":{}}}` + "\n")
	output := &bytes.Buffer{}

	transport := NewStdioTransportWithIO(server, logger, input, output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		transport.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	var response JSONRPCResponse
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	resultBytes, _ := json.Marshal(response.Result)
	var callResult ToolsCallResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal call result: %v", err)
	}

	if len(callResult.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(callResult.Content))
	}

	if callResult.Content[0].Text != systemMsg {
		t.Errorf("expected text '%s', got %s", systemMsg, callResult.Content[0].Text)
	}
}
