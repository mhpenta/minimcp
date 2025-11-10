// Package tools provides interfaces and utilities for creating MCP tools.
//
// This package defines the core Tool interface and provides TypedTool, a
// type-safe abstraction for creating tools with automatic schema generation
// and safe JSON unmarshalling.
//
// # Basic Usage
//
// Create a typed tool using NewTool:
//
//	type WeatherRequest struct {
//	    City string `json:"city"`
//	}
//
//	type WeatherResponse struct {
//	    Temperature float64 `json:"temperature"`
//	    Conditions  string  `json:"conditions"`
//	}
//
//	func getWeather(ctx context.Context, req WeatherRequest) (WeatherResponse, error) {
//	    // implementation
//	    return WeatherResponse{Temperature: 22.5, Conditions: "Sunny"}, nil
//	}
//
//	tool := tools.NewTool(
//	    "get_weather",
//	    "Fetches current weather information",
//	    getWeather,
//	)
//
// # Manual Tool Implementation
//
// For more control, implement the Tool interface directly:
//
//	type MyTool struct{}
//
//	func (t *MyTool) Spec() *tools.ToolSpec {
//	    return &tools.ToolSpec{
//	        Name:        "my_tool",
//	        Description: "Does something useful",
//	        Parameters:  map[string]interface{}{ /* JSON schema */ },
//	    }
//	}
//
//	func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
//	    // implementation
//	}
//
// # Tool Options
//
// Customize tool behavior with functional options:
//
//	tool := tools.NewTool(
//	    "my_tool",
//	    "Description",
//	    handler,
//	    tools.WithVerb("Processing"),
//	    tools.WithLongRunning(true),
//	    tools.WithType("custom_type"),
//	)
//
// # Error Handling
//
// NewTool panics on schema generation errors (fail-fast at initialization).
// Use NewToolWithError for explicit error handling:
//
//	tool, err := tools.NewToolWithError(name, description, handler)
//	if err != nil {
//	    // handle error
//	}
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool defines the interface that all tools must implement
type Tool interface {
	// Spec returns the tool's specification, including name, description, parameters, and UI hints.
	Spec() *ToolSpec

	// Execute runs the tool with given parameters
	Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error)
}

type ToolSpec struct {
	// Name returns the tool's identifier
	Name string `json:"name,omitempty"`

	// Type returns the tool's type, which is used for categorization
	Type string `json:"type,omitempty"`

	// Description returns the tool's description
	Description string `json:"description,omitempty"`

	// Parameters returns the tool's parameter schema
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// Output returns the tool's output schema
	Output map[string]interface{} `json:"output,omitempty"`

	// Sequential indicates if a tool must be run sequentially with other tools. False means we can run it in parallel.
	Sequential bool `json:"sequential,omitempty"`

	// UI provides additional UI hints for the tool
	UI UI `json:"ui,omitempty"`
}

type UI struct {
	// Verb is a present progressive verb phrase for UI display (e.g., "Searching for companies")
	Verb string `json:"verb,omitempty"`

	// LongRunning indicates if the tool is expected to run for a long time, resulting in different handling in the UI
	LongRunning bool `json:"long_running,omitempty"`
}

const (
	maxToolNameLength = 64
)

func Validate(t Tool) error {
	if t == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	m := t.Spec()
	if m.Name == "" {
		return fmt.Errorf("tool spec must include a non-empty name")
	}

	if len(m.Name) > maxToolNameLength {
		return fmt.Errorf("tool name must not exceed 64 characters")
	}

	for _, char := range m.Name {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '-' {
			continue
		}
		return fmt.Errorf("tool name must contain only alphanumeric characters, underscores, or hyphens")
	}

	if m.Description == "" {
		return fmt.Errorf("tool spec description cannot be empty")
	}

	if m.Parameters == nil {
		return fmt.Errorf("tool spec parameters cannot be nil")
	}

	return nil
}
