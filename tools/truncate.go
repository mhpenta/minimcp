package tools

import (
	"fmt"
	"unicode/utf8"
)

// TruncateResponse truncates a response to maxBytes while preserving UTF-8 validity
// and adds a truncation indicator for LLM recovery
func TruncateResponse(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return text // No limit configured
	}

	textBytes := []byte(text)
	originalLen := len(textBytes)

	if originalLen <= maxBytes {
		return text // No truncation needed
	}

	// Calculate space needed for truncation indicator
	indicatorTemplate := "\n\n[TRUNCATED: original_size=%d bytes, showing=%d bytes, truncated=%d bytes]"
	truncatedBytes := originalLen - maxBytes
	indicator := fmt.Sprintf(indicatorTemplate, originalLen, maxBytes, truncatedBytes)
	indicatorLen := len([]byte(indicator))

	truncateAt := maxBytes - indicatorLen
	if truncateAt < 0 {
		truncateAt = 0
	}

	// Truncate while ensuring UTF-8 validity - don't cut in the middle of a multi-byte character
	for truncateAt > 0 && !utf8.Valid(textBytes[:truncateAt]) {
		truncateAt--
	}

	truncated := string(textBytes[:truncateAt])
	actualShowing := len([]byte(truncated))
	truncatedAmount := originalLen - actualShowing

	finalIndicator := fmt.Sprintf(
		"\n\n[TRUNCATED: original_size=%d bytes, showing=%d bytes, truncated=%d bytes]",
		originalLen,
		actualShowing,
		truncatedAmount,
	)

	return truncated + finalIndicator
}
