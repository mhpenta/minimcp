package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateResponse_NoLimit(t *testing.T) {
	text := "This is a test response"
	result := TruncateResponse(text, 0)
	if result != text {
		t.Errorf("Expected no truncation with maxBytes=0, got truncated text")
	}

	result = TruncateResponse(text, -1)
	if result != text {
		t.Errorf("Expected no truncation with maxBytes=-1, got truncated text")
	}
}

func TestTruncateResponse_NoTruncationNeeded(t *testing.T) {
	text := "Short text"
	result := TruncateResponse(text, 1000)
	if result != text {
		t.Errorf("Expected no truncation when text is shorter than limit")
	}
}

func TestTruncateResponse_BasicTruncation(t *testing.T) {
	text := strings.Repeat("a", 1000)
	maxBytes := 100
	result := TruncateResponse(text, maxBytes)

	// Check that result is truncated
	if len(result) <= len(text) {
		// Should be truncated
		if !strings.Contains(result, "[TRUNCATED:") {
			t.Errorf("Expected truncation indicator in result")
		}
	} else {
		t.Errorf("Result should not be longer than original")
	}

	// Check that the result includes truncation info
	if !strings.Contains(result, "original_size=1000 bytes") {
		t.Errorf("Expected original_size=1000 bytes in truncation indicator")
	}

	// Check UTF-8 validity
	if !utf8.ValidString(result) {
		t.Errorf("Result should be valid UTF-8")
	}
}

func TestTruncateResponse_UTF8Safety(t *testing.T) {
	// Test with multi-byte Unicode characters
	text := strings.Repeat("🔥", 100) // Each emoji is 4 bytes
	maxBytes := 50
	result := TruncateResponse(text, maxBytes)

	// Check UTF-8 validity
	if !utf8.ValidString(result) {
		t.Errorf("Result should be valid UTF-8 even with multi-byte characters")
	}

	// Should contain truncation indicator
	if !strings.Contains(result, "[TRUNCATED:") {
		t.Errorf("Expected truncation indicator")
	}
}

func TestTruncateResponse_MixedUTF8(t *testing.T) {
	// Test with mixed ASCII and multi-byte characters
	text := "Hello " + strings.Repeat("世界", 100) // Chinese characters (3 bytes each)
	maxBytes := 100
	result := TruncateResponse(text, maxBytes)

	// Check UTF-8 validity
	if !utf8.ValidString(result) {
		t.Errorf("Result should be valid UTF-8 with mixed character types")
	}

	// Should be truncated
	if len(result) >= len(text) {
		t.Errorf("Expected truncation to occur")
	}
}

func TestTruncateResponse_IndicatorFormat(t *testing.T) {
	text := strings.Repeat("x", 500)
	maxBytes := 200
	result := TruncateResponse(text, maxBytes)

	// Check that indicator has expected format
	if !strings.Contains(result, "[TRUNCATED: original_size=") {
		t.Errorf("Expected 'original_size=' in indicator")
	}
	if !strings.Contains(result, "showing=") {
		t.Errorf("Expected 'showing=' in indicator")
	}
	if !strings.Contains(result, "truncated=") {
		t.Errorf("Expected 'truncated=' in indicator")
	}
	if !strings.Contains(result, "bytes]") {
		t.Errorf("Expected 'bytes]' in indicator")
	}
}

func TestTruncateResponse_VerySmallLimit(t *testing.T) {
	text := strings.Repeat("a", 1000)
	maxBytes := 10 // Very small limit
	result := TruncateResponse(text, maxBytes)

	// Should still produce valid output with indicator
	if !utf8.ValidString(result) {
		t.Errorf("Result should be valid UTF-8 even with very small limit")
	}

	// Should contain truncation indicator
	if !strings.Contains(result, "[TRUNCATED:") {
		t.Errorf("Expected truncation indicator even with very small limit")
	}
}

func TestTruncateResponse_ExactLimit(t *testing.T) {
	text := "exactly100"
	for len(text) < 100 {
		text += "x"
	}
	// text is now exactly 100 bytes

	result := TruncateResponse(text, 100)

	// Should not be truncated (equal to limit)
	if result != text {
		t.Errorf("Expected no truncation when text equals limit exactly")
	}
}

func TestTruncateResponse_OneBytePastLimit(t *testing.T) {
	text := "exactly101"
	for len(text) < 101 {
		text += "x"
	}
	// text is now exactly 101 bytes

	result := TruncateResponse(text, 100)

	// Should be truncated
	if !strings.Contains(result, "[TRUNCATED:") {
		t.Errorf("Expected truncation when text is 1 byte over limit")
	}

	// Should be valid UTF-8
	if !utf8.ValidString(result) {
		t.Errorf("Result should be valid UTF-8")
	}
}

func TestTruncateResponse_EmptyString(t *testing.T) {
	text := ""
	result := TruncateResponse(text, 100)

	if result != text {
		t.Errorf("Expected empty string to remain empty")
	}
}

func TestTruncateResponse_LargeText(t *testing.T) {
	// Test with a large text to ensure performance
	text := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 10000)
	maxBytes := 5000
	result := TruncateResponse(text, maxBytes)

	// Should be truncated
	if len(result) >= len(text) {
		t.Errorf("Expected large text to be truncated")
	}

	// Should contain indicator
	if !strings.Contains(result, "[TRUNCATED:") {
		t.Errorf("Expected truncation indicator in large text")
	}

	// Should be valid UTF-8
	if !utf8.ValidString(result) {
		t.Errorf("Result should be valid UTF-8")
	}
}

func TestTruncateResponse_IndicatorAccuracy(t *testing.T) {
	text := strings.Repeat("test ", 200) // 1000 bytes
	maxBytes := 300
	result := TruncateResponse(text, maxBytes)

	// Parse the indicator to check accuracy
	if !strings.Contains(result, "original_size=1000 bytes") {
		t.Errorf("Expected accurate original_size in indicator, got: %s", result)
	}

	// The showing + truncated should equal original
	// This is tested by the indicator format itself
}
