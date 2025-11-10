package infer

import (
	"context"
	"testing"
)

// Test types
type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	ZipCode string `json:"zip_code"`
}

type ComplexType struct {
	ID       int               `json:"id"`
	Person   Person            `json:"person"`
	Address  Address           `json:"address"`
	Tags     []string          `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

func TestFromFunc_ValidFunction(t *testing.T) {
	handler := func(ctx context.Context, input Person) (Address, error) {
		return Address{}, nil
	}

	inputSchema, outputSchema, err := FromFunc(handler)
	if err != nil {
		t.Fatalf("FromFunc failed: %v", err)
	}

	if inputSchema == nil {
		t.Fatal("Expected non-nil input schema")
	}

	if outputSchema == nil {
		t.Fatal("Expected non-nil output schema")
	}

	// Verify input schema
	inputMap, err := ToMap(inputSchema)
	if err != nil {
		t.Fatalf("ToMap for input failed: %v", err)
	}

	props, ok := inputMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected input properties to be a map")
	}

	if _, ok := props["name"]; !ok {
		t.Error("Expected 'name' in input schema")
	}

	// Verify output schema
	outputMap, err := ToMap(outputSchema)
	if err != nil {
		t.Fatalf("ToMap for output failed: %v", err)
	}

	props, ok = outputMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected output properties to be a map")
	}

	if _, ok := props["street"]; !ok {
		t.Error("Expected 'street' in output schema")
	}
}

func TestFromFuncInput_ValidFunction(t *testing.T) {
	handler := func(ctx context.Context, input Person) (Address, error) {
		return Address{}, nil
	}

	inputSchema, err := FromFuncInput(handler)
	if err != nil {
		t.Fatalf("FromFuncInput failed: %v", err)
	}

	if inputSchema == nil {
		t.Fatal("Expected non-nil input schema")
	}

	inputMap, err := ToMap(inputSchema)
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}

	if inputMap["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", inputMap["type"])
	}
}

func TestToMap_NilSchema(t *testing.T) {
	_, err := ToMap(nil)
	if err == nil {
		t.Fatal("Expected error for nil schema")
	}
}
