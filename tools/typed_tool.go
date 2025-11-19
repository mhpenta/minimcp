package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mhpenta/minimcp/infer"
	"github.com/mhpenta/minimcp/safeunmarshal"
)

type TypedTool[In, Out any] struct {
	spec    *ToolSpec
	handler func(context.Context, In) (Out, error)
}

func (t *TypedTool[In, Out]) Spec() *ToolSpec {
	return t.spec
}

func (t *TypedTool[In, Out]) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var input In
	if len(params) > 0 {
		parsedInput, err := safeunmarshal.To[In](params)
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("failed to parse parameters: %v", err))
		}
		input = parsedInput
	}
	result, err := t.handler(ctx, input)
	if err != nil {
		return nil, err
	}
	return &ToolResult{
		Output: result,
		Error:  nil,
	}, nil
}

// ToolOption for functional configuration
type ToolOption func(*ToolSpec)

func WithType(toolType string) ToolOption {
	return func(spec *ToolSpec) {
		spec.Type = toolType
	}
}

func WithVerb(verb string) ToolOption {
	return func(spec *ToolSpec) {
		spec.UI.Verb = verb
	}
}

func WithLongRunning(longRunning bool) ToolOption {
	return func(spec *ToolSpec) {
		spec.UI.LongRunning = longRunning
	}
}

func WithCustomSchema(schema map[string]interface{}) ToolOption {
	return func(spec *ToolSpec) {
		spec.Parameters = schema
	}
}

// NewTool creates a new TypedTool with automatic schema generation and safe unmarshalling.
// It panics if schema generation fails, following the principle of failing fast at initialization time.
// For more control over error handling, use NewToolWithError.
//
// Example:
//
//	tool := tools.NewTool(
//	    "get_weather",
//	    "Fetches weather information",
//	    handleWeather,
//	    tools.WithVerb("Fetching weather"),
//	)
func NewTool[In, Out any](
	name,
	description string,
	handler func(context.Context, In) (Out, error),
	opts ...ToolOption,
) Tool {
	tool, err := NewToolWithError[In, Out](name, description, handler, opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create tool %q: %v", name, err))
	}
	return tool
}

// NewToolWithError creates a new TypedTool with automatic schema generation and safe unmarshalling,
// returning an error instead of panicking on failure.
// Use this when you need more control over error handling at initialization time.
//
// Example:
//
//	tool, err := tools.NewToolWithError(
//	    "get_weather",
//	    "Fetches weather information",
//	    handleWeather,
//	)
//	if err != nil {
//	    return fmt.Errorf("failed to create weather tool: %w", err)
//	}
func NewToolWithError[In, Out any](
	name,
	description string,
	handler func(context.Context, In) (Out, error),
	opts ...ToolOption,
) (Tool, error) {

	inputSchema, outputSchema, err := infer.FromFunc(handler)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema from handler function: %w", err)
	}

	inputSchemaMap, err := infer.ToMap(inputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert input schema to map: %w", err)
	}

	outputSchemaMap, err := infer.ToMap(outputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output schema to map: %w", err)
	}

	spec := &ToolSpec{
		Name:        name,
		Type:        fmt.Sprintf("%s_v1", name),
		Description: description,
		Parameters:  inputSchemaMap,
		Output:      outputSchemaMap,
		Sequential:  false,
		UI:          UI{},
	}

	for _, opt := range opts {
		opt(spec)
	}

	return &TypedTool[In, Out]{
		spec:    spec,
		handler: handler,
	}, nil
}
