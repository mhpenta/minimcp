// Package safeunmarshal provides utilities for safely unmarshalling JSON data.
// It includes functionality to handle malformed JSON, extract JSON from text,
// and repair common JSON formatting issues.
package safeunmarshal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

const (
	// DefaultMaxInputSize is the default maximum size for JSON input (10MB)
	DefaultMaxInputSize = 10 * 1024 * 1024
)

// UnmarshalOptions configures the behavior of JSON unmarshalling.
type UnmarshalOptions struct {
	// MaxInputSize is the maximum allowed size for input JSON in bytes.
	// Set to 0 for no limit. Default is 10MB.
	MaxInputSize int

	// EnableRepair enables automatic JSON repair for malformed input.
	// When false, only well-formed JSON will be accepted. Default is true for backwards compatibility.
	EnableRepair bool
}

// DefaultOptions returns the default unmarshalling options.
func DefaultOptions() UnmarshalOptions {
	return UnmarshalOptions{
		MaxInputSize: DefaultMaxInputSize,
		EnableRepair: true,
	}
}

// StrictOptions returns strict unmarshalling options (no repair, size limits enabled).
func StrictOptions() UnmarshalOptions {
	return UnmarshalOptions{
		MaxInputSize: DefaultMaxInputSize,
		EnableRepair: false,
	}
}

// ToLenient attempts to unmarshal a JSON byte slice into a value of type T.
// This is the lenient version that enables JSON repair by default.
//
// This function can handle various types, including arrays and slices. It first attempts
// to unmarshal the provided JSON directly. If that fails, it tries to repair the JSON before
// attempting to unmarshal again.
//
// For more control over the unmarshalling behavior, use ToWithOptions.
// For strict mode (no repair), use ToStrict.
//
// Parameters:
//   - raw: A byte slice containing the JSON data to be parsed.
//
// Returns:
//   - T: The unmarshalled value of type T.
//   - error: An error if the unmarshalling process fails, or nil if successful.
//     Notably, it returns ErrExpectedJSONArray (wrapped in a fmt.Errorf) if the
//     target type is an array or slice but the input is not a JSON array.
//
// Usage:
//
//	type MyStruct struct {
//	    Field string `json:"field"`
//	}
//	jsonData := []byte(`{"field": "value"}`)
//	result, err := safeunmarshal.ToLenient[MyStruct](jsonData)
//	if err != nil {
//	    if errors.Is(err, ErrExpectedJSONArray) {
//	        // Handle case where array was expected but not received
//	    } else {
//	        // Handle other errors
//	    }
//	}
func ToLenient[T any](raw []byte) (T, error) {
	return ToWithOptions[T](raw, DefaultOptions())
}

// To attempts to unmarshal a JSON byte slice into a value of type T using strict mode.
// Strict mode disables JSON repair and only accepts well-formed JSON.
//
// Use this when you want to ensure that only valid JSON is processed and avoid
// potentially masking bugs with automatic repair.
//
// Parameters:
//   - raw: A byte slice containing the JSON data to be parsed.
//
// Returns:
//   - T: The unmarshalled value of type T.
//   - error: An error if the unmarshalling process fails, or nil if successful.
//
// Usage:
//
//	result, err := safeunmarshal.ToStrict[MyStruct](jsonData)
//	if err != nil {
//	    // Handle error - no automatic repair was attempted
//	}
func To[T any](raw []byte) (T, error) {
	return ToWithOptions[T](raw, StrictOptions())
}

// ToWithOptions attempts to unmarshal a JSON byte slice into a value of type T with custom options.
//
// Parameters:
//   - raw: A byte slice containing the JSON data to be parsed.
//   - opts: Unmarshalling options to control behavior.
//
// Returns:
//   - T: The unmarshalled value of type T.
//   - error: An error if the unmarshalling process fails, or nil if successful.
//
// Usage:
//
//	opts := safeunmarshal.UnmarshalOptions{
//	    MaxInputSize: 1024 * 1024, // 1MB limit
//	    EnableRepair: false,        // strict mode
//	}
//	result, err := safeunmarshal.ToWithOptions[MyStruct](jsonData, opts)
func ToWithOptions[T any](raw []byte, opts UnmarshalOptions) (T, error) {
	var zero T // original zero value to return in case of error

	// Check input size limit
	if opts.MaxInputSize > 0 && len(raw) > opts.MaxInputSize {
		return zero, fmt.Errorf("input size %d exceeds maximum allowed size %d", len(raw), opts.MaxInputSize)
	}

	data := prepareJSONForUnmarshalling(raw)
	data = bytes.ReplaceAll(data, []byte("\n"), []byte(""))

	if len(data) == 0 {
		return zero, fmt.Errorf("empty input string")
	}

	var response T
	err := json.Unmarshal(data, &response)
	if err != nil {
		valueType := reflect.TypeOf((*T)(nil)).Elem()
		isArray := valueType.Kind() == reflect.Array || valueType.Kind() == reflect.Slice

		if isArray && !isJSONArray(data) {
			return zero, fmt.Errorf("%w: got %s", ErrExpectedJSONArray, data)
		}

		// Only attempt repair if enabled in options
		if !opts.EnableRepair {
			return zero, fmt.Errorf("failed to parse JSON: %w", err)
		}

		repairedData, repairErr := repairJSON(string(data))
		if repairErr != nil {
			return zero, fmt.Errorf("failed to repair JSON: %w", repairErr)
		}

		if repairedData == "" {
			return zero, fmt.Errorf("JSON repair resulted in empty string")
		}

		err = json.Unmarshal([]byte(repairedData), &response)
		if err != nil {
			return zero, fmt.Errorf("failed to parse repaired JSON: %w", err)
		}
	}
	return response, nil
}

// isJSONArray checks if the input byte slice represents a JSON array.
//
// This function scans the input byte slice, skipping any leading whitespace,
// to determine if it starts with an opening square bracket '[', which
// indicates the beginning of a JSON array.
//
// Parameters:
//   - data: A byte slice containing the JSON data to be checked.
//
// Returns:
//   - bool: true if the input represents a JSON array, false otherwise.
//
// Note: This function only checks the first non-whitespace character
// and does not validate the entire JSON structure.
func isJSONArray(data []byte) bool {
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b == '['
	}
	return false
}

// prepareJSONForUnmarshalling extracts JSON from data, handling cases where
// JSON might be embedded in text. It looks for the first complete JSON object
// or array in the input.
func prepareJSONForUnmarshalling(data []byte) []byte {
	trimmedData := bytes.TrimSpace(data)

	if len(trimmedData) == 0 {
		return nil
	}

	// Check if we have a complete JSON object or array already
	if (trimmedData[0] == '{' && trimmedData[len(trimmedData)-1] == '}') ||
		(trimmedData[0] == '[' && trimmedData[len(trimmedData)-1] == ']') {
		return trimmedData
	}

	// Try to extract JSON object first
	startIndex := -1
	braceCount := 0
	for i, char := range data {
		if char == '{' {
			braceCount++
			if startIndex == -1 {
				startIndex = i
			}
		} else if char == '}' {
			braceCount--
			if braceCount == 0 && startIndex != -1 {
				return data[startIndex : i+1]
			}
		}
	}

	// Try to extract JSON array if object extraction failed
	startIndex = -1
	bracketCount := 0
	for i, char := range data {
		if char == '[' {
			bracketCount++
			if startIndex == -1 {
				startIndex = i
			}
		} else if char == ']' {
			bracketCount--
			if bracketCount == 0 && startIndex != -1 {
				return data[startIndex : i+1]
			}
		}
	}

	// No valid JSON found
	return nil
}
