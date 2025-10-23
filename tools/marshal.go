package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// MarshalOutput converts an input object to its JSON string representation and removes surrounding quotes if present.
func MarshalOutput(logger *slog.Logger, o any) string {
	if str, ok := o.(string); ok {
		return str
	}

	outputBytes, err := json.Marshal(o)
	if err != nil {
		logger.Error("Error marshalling output",
			"error", err,
			"type", fmt.Sprintf("%T", o),
			"value", fmt.Sprintf("%+v", o))
		return ""
	}

	if len(outputBytes) > 1 && outputBytes[0] == '"' && outputBytes[len(outputBytes)-1] == '"' {
		return string(outputBytes[1 : len(outputBytes)-1])
	}

	return string(outputBytes)
}
