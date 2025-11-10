package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// Test types
type TestInput struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type TestOutput struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
}

// Test handler function
func testHandler(ctx context.Context, input TestInput) (TestOutput, error) {
	return TestOutput{
		Result:  "processed: " + input.Name,
		Success: true,
	}, nil
}

func errorHandler(ctx context.Context, input TestInput) (TestOutput, error) {
	return TestOutput{}, errors.New("handler error")
}

func TestNewTool_Success(t *testing.T) {
	tool := NewTool(
		"test_tool",
		"A test tool",
		testHandler,
	)

	if tool == nil {
		t.Fatal("NewTool returned nil")
	}

	spec := tool.Spec()
	if spec == nil {
		t.Fatal("Spec() returned nil")
	}

	if spec.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got %q", spec.Name)
	}

	if spec.Description != "A test tool" {
		t.Errorf("Expected description 'A test tool', got %q", spec.Description)
	}

	if spec.Parameters == nil {
		t.Error("Parameters should not be nil")
	}

	if spec.Output == nil {
		t.Error("Output should not be nil")
	}
}

func TestNewTool_WithOptions(t *testing.T) {
	tool := NewTool(
		"test_tool",
		"A test tool",
		testHandler,
		WithVerb("Testing"),
		WithLongRunning(true),
		WithType("custom_type"),
	)

	spec := tool.Spec()
	if spec.UI.Verb != "Testing" {
		t.Errorf("Expected verb 'Testing', got %q", spec.UI.Verb)
	}

	if !spec.UI.LongRunning {
		t.Error("Expected LongRunning to be true")
	}

	if spec.Type != "custom_type" {
		t.Errorf("Expected type 'custom_type', got %q", spec.Type)
	}
}

func TestNewToolWithError_Success(t *testing.T) {
	tool, err := NewToolWithError(
		"test_tool",
		"A test tool",
		testHandler,
	)

	if err != nil {
		t.Fatalf("NewToolWithError returned unexpected error: %v", err)
	}

	if tool == nil {
		t.Fatal("NewToolWithError returned nil tool")
	}

	spec := tool.Spec()
	if spec.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got %q", spec.Name)
	}
}

func TestTypedTool_Execute_Success(t *testing.T) {
	tool := NewTool(
		"test_tool",
		"A test tool",
		testHandler,
	)

	input := TestInput{
		Name:  "test",
		Value: 42,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Execute returned nil result")
	}

	output, ok := result.Output.(TestOutput)
	if !ok {
		t.Fatalf("Expected TestOutput, got %T", result.Output)
	}

	if output.Result != "processed: test" {
		t.Errorf("Expected result 'processed: test', got %q", output.Result)
	}

	if !output.Success {
		t.Error("Expected Success to be true")
	}
}

func TestTypedTool_Execute_HandlerError(t *testing.T) {
	tool := NewTool(
		"error_tool",
		"A tool that errors",
		errorHandler,
	)

	input := TestInput{Name: "test", Value: 42}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	_, err = tool.Execute(context.Background(), inputJSON)
	if err == nil {
		t.Fatal("Expected error from handler, got nil")
	}

	if err.Error() != "handler error" {
		t.Errorf("Expected error 'handler error', got %q", err.Error())
	}
}

func TestTypedTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewTool(
		"test_tool",
		"A test tool",
		testHandler,
	)

	invalidJSON := json.RawMessage(`{"name": "test", "value": "not a number"}`)

	_, err := tool.Execute(context.Background(), invalidJSON)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestTypedTool_Execute_EmptyInput(t *testing.T) {
	emptyHandler := func(ctx context.Context, input struct{}) (TestOutput, error) {
		return TestOutput{Result: "empty input ok", Success: true}, nil
	}

	tool := NewTool(
		"empty_tool",
		"A tool with empty input",
		emptyHandler,
	)

	result, err := tool.Execute(context.Background(), json.RawMessage{})
	if err != nil {
		t.Fatalf("Execute with empty input returned error: %v", err)
	}

	output, ok := result.Output.(TestOutput)
	if !ok {
		t.Fatalf("Expected TestOutput, got %T", result.Output)
	}

	if output.Result != "empty input ok" {
		t.Errorf("Expected result 'empty input ok', got %q", output.Result)
	}
}

func TestTypedTool_Execute_MalformedJSON(t *testing.T) {
	tool := NewTool(
		"test_tool",
		"A test tool",
		testHandler,
	)

	// safeunmarshal should handle this if repair is enabled
	malformedJSON := json.RawMessage(`Some text before {"name": "test", "value": 42}`)

	result, err := tool.Execute(context.Background(), malformedJSON)
	if err != nil {
		t.Fatalf("Execute with malformed JSON returned error: %v", err)
	}

	output, ok := result.Output.(TestOutput)
	if !ok {
		t.Fatalf("Expected TestOutput, got %T", result.Output)
	}

	if output.Result != "processed: test" {
		t.Errorf("Expected result 'processed: test', got %q", output.Result)
	}
}

func TestWithCustomSchema(t *testing.T) {
	customSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"custom_field": map[string]interface{}{
				"type": "string",
			},
		},
	}

	tool := NewTool(
		"test_tool",
		"A test tool",
		testHandler,
		WithCustomSchema(customSchema),
	)

	spec := tool.Spec()
	if spec.Parameters == nil {
		t.Fatal("Parameters should not be nil")
	}

	props, ok := spec.Parameters["properties"]
	if !ok {
		t.Fatal("Parameters should have 'properties' field")
	}

	propsMap, ok := props.(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	if _, ok := propsMap["custom_field"]; !ok {
		t.Error("Custom schema should include 'custom_field'")
	}
}
