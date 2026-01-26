package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// MarshalOutput converts an input object to its JSON string representation and removes surrounding quotes if present.
// If maxResponseBytes > 0, responses exceeding this size will be truncated with an indicator.
func MarshalOutput(logger *slog.Logger, o any, maxResponseBytes int) string {
	var text string

	if str, ok := o.(string); ok {
		text = str
	} else {
		outputBytes, err := json.Marshal(o)
		if err != nil {
			logger.Error("Error marshalling output",
				"error", err,
				"type", fmt.Sprintf("%T", o),
				"value", fmt.Sprintf("%+v", o))
			return ""
		}

		if len(outputBytes) > 1 && outputBytes[0] == '"' && outputBytes[len(outputBytes)-1] == '"' {
			text = string(outputBytes[1 : len(outputBytes)-1])
		} else {
			text = string(outputBytes)
		}
	}

	// Apply truncation if configured
	return TruncateResponse(text, maxResponseBytes)
}
