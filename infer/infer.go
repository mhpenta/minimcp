// Package infer provides utilities for automatic JSON schema generation from Go types.
//
// This package is a convenience wrapper around github.com/google/jsonschema-go that
// provides a clean, type-safe API for generating JSON schemas from Go types and
// function signatures. It's designed specifically for use with MCP tools but can
// be used in any context requiring JSON schema generation.
//
// # Basic Usage
//
// Generate a schema from a Go type:
//
//	type User struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//
//	schema, err := infer.FromType[User]()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Generate schemas from a function signature:
//
//	func HandleUser(ctx context.Context, req UserRequest) (UserResponse, error) {
//	    // implementation
//	}
//
//	inputSchema, outputSchema, err := infer.FromFunc(HandleUser)
//
// Convert schema to map for JSON encoding:
//
//	schemaMap, err := infer.ToMap(schema)
//
// # Schema Customization
//
// Schemas can be customized using struct tags. The package supports standard
// `json` tags and can be extended with schema validation tags supported by
// google/jsonschema-go. See https://github.com/google/jsonschema-go for
// full documentation on supported tags and options.
//
// # Type Safety
//
// All functions use Go generics to provide compile-time type safety. The type
// parameter T in FromType[T]() and similar functions ensures that schema
// generation is type-checked at compile time.
package infer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/jsonschema-go/jsonschema"
)

// FromType generates a JSON schema from a Go type using type inference.
// This is a convenience wrapper around jsonschema.For that fits the infer API.
//
// Example:
//
//	type User struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//
//	schema, err := infer.FromType[User]()
func FromType[T any]() (*jsonschema.Schema, error) {
	return jsonschema.For[T](nil)
}

// FromFunc generates input and output JSON schemas from a function signature.
// The function must have the signature: func(context.Context, T) (R, error)
// where T and R are the input and output types respectively.
//
// Example:
//
//	func HandleUser(ctx context.Context, req UserRequest) (UserResponse, error) {
//	    // handler code
//	}
//
//	input, output, err := schematic.FromFunc(HandleUser)
func FromFunc[T any, R any](fn func(context.Context, T) (R, error)) (*jsonschema.Schema, *jsonschema.Schema, error) {
	// Generate input schema
	inputSchema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, nil, fmt.Errorf("generating input schema: %w", err)
	}

	// Generate output schema
	outputSchema, err := jsonschema.For[R](nil)
	if err != nil {
		return nil, nil, fmt.Errorf("generating output schema: %w", err)
	}

	return inputSchema, outputSchema, nil
}

// FromFuncInput generates only the input schema from a function signature.
// The function must have the signature: func(context.Context, T) (R, error)
//
// Example:
//
//	input, err := schematic.FromFuncInput(HandleUser)
func FromFuncInput[T any, R any](fn func(context.Context, T) (R, error)) (*jsonschema.Schema, error) {
	return jsonschema.For[T](nil)
}

// ToMap converts a jsonschema.Schema to a map[string]interface{} representation.
// This is useful when you want to work with the schema as a plain map
// or integrate it with systems that expect map-based data structures.
//
// This is structured to marshal and then unmarshal to ensure fidelity, given custom marshalling in jsonschema.
//
// Example:
//
//	schema, _ := FromType[User]()
//	schemaMap, err := ToMap(schema)
func ToMap(s *jsonschema.Schema) (map[string]interface{}, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot convert nil schema to map")
	}

	data, err := s.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema to map: %w", err)
	}

	return result, nil
}
