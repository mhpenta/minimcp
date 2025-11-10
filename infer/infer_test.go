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
	ID       int       `json:"id"`
	Person   Person    `json:"person"`
	Address  Address   `json:"address"`
	Tags     []string  `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

type EmptyStruct struct{}

func TestFromType_SimpleStruct(t *testing.T) {
	schema, err := FromType[Person]()
	if err != nil {
		t.Fatalf("FromType failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	// Verify we can convert to map
	schemaMap, err := ToMap(schema)
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}

	if schemaMap["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", schemaMap["type"])
	}

	props, ok := schemaMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	if _, ok := props["name"]; !ok {
		t.Error("Expected 'name' property")
	}

	if _, ok := props["age"]; !ok {
		t.Error("Expected 'age' property")
	}
}

func TestFromType_ComplexStruct(t *testing.T) {
	schema, err := FromType[ComplexType]()
	if err != nil {
		t.Fatalf("FromType failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	schemaMap, err := ToMap(schema)
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}

	props, ok := schemaMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	// Check nested struct
	if _, ok := props["person"]; !ok {
		t.Error("Expected 'person' property")
	}

	// Check array
	if _, ok := props["tags"]; !ok {
		t.Error("Expected 'tags' property")
	}

	// Check map
	if _, ok := props["metadata"]; !ok {
		t.Error("Expected 'metadata' property")
	}
}

func TestFromType_EmptyStruct(t *testing.T) {
	schema, err := FromType[EmptyStruct]()
	if err != nil {
		t.Fatalf("FromType failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}
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

func TestToMap_ValidSchema(t *testing.T) {
	schema, err := FromType[Person]()
	if err != nil {
		t.Fatalf("FromType failed: %v", err)
	}

	schemaMap, err := ToMap(schema)
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}

	if schemaMap == nil {
		t.Fatal("Expected non-nil map")
	}

	// Verify it's a proper map with expected structure
	if _, ok := schemaMap["type"]; !ok {
		t.Error("Expected 'type' field in schema map")
	}

	if _, ok := schemaMap["properties"]; !ok {
		t.Error("Expected 'properties' field in schema map")
	}
}

// Test that schemas are properly generated for primitive types wrapped in structs
func TestFromType_PrimitiveTypes(t *testing.T) {
	type StringWrapper struct {
		Value string `json:"value"`
	}

	type IntWrapper struct {
		Value int `json:"value"`
	}

	type BoolWrapper struct {
		Value bool `json:"value"`
	}

	tests := []struct {
		name     string
		typeTest func() error
	}{
		{
			name: "string type",
			typeTest: func() error {
				_, err := FromType[StringWrapper]()
				return err
			},
		},
		{
			name: "int type",
			typeTest: func() error {
				_, err := FromType[IntWrapper]()
				return err
			},
		},
		{
			name: "bool type",
			typeTest: func() error {
				_, err := FromType[BoolWrapper]()
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.typeTest()
			if err != nil {
				t.Errorf("FromType failed for %s: %v", tt.name, err)
			}
		})
	}
}
