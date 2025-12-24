package utilitytools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mhpenta/minimcp/tools"
)

// AdminScreenshotToolParams defines parameters for capturing screenshots
type AdminScreenshotToolParams struct {
	Interactive bool   `json:"interactive" desc:"Enable interactive mode to select area/window" required:"false"`
	CameraNoise bool   `json:"cameraNoise" desc:"Enable camera sound (default: silent)" required:"false"`
	Delay       int    `json:"delay" desc:"Delay in seconds before capturing (default: 0)" required:"false"`
	Filename    string `json:"filename" desc:"Custom filename (optional, auto-generated if not provided)" required:"false"`
}

// ScreenShotTool defines the result of a screenshot capture
type ScreenShotTool struct {
	Success  bool   `json:"success"`
	FilePath string `json:"filePath"`
	Message  string `json:"message"`
}

const screenshotToolDescription = `Captures a screenshot and saves it to the local filesystem.

This tool uses the native macOS 'screencapture' utility to capture screenshots. It only works on macOS systems, will 
return an error on other operating systems.

When a user asks for a screen shot, assume they want to take an interactive one by default.

PARAMETERS:
- interactive (optional): Enable interactive mode to select a specific area or window (default: false)
- cameraNoise (optional): Enable camera shutter sound (default: false/silent)
- delay (optional): Delay in seconds before capturing the screenshot (default: 0)
- filename (optional): Custom filename without extension (auto-generated timestamp if not provided)

BEHAVIOR:
- Screenshots are saved to ./screenshots/ directory relative to the project root

RETURNS:
- success: Whether the screenshot was captured successfully
- filePath: Full path to the saved screenshot file
- message: Status message or error description`

// NewScreenShotTool creates a new screenshot tool using the NewTool pattern
func NewScreenShotTool(projectBasePath string) tools.Tool {
	handler := func(ctx context.Context, params AdminScreenshotToolParams) (*ScreenShotTool, error) {
		// Check if screenshots are supported on this OS
		if !CanScreenshot() {
			return &ScreenShotTool{
				Success: false,
				Message: "Screenshot functionality is only available on macOS",
			}, nil
		}

		// Create screenshots directory if it doesn't exist
		screenshotDir := filepath.Join(projectBasePath, "screenshots")
		if err := os.MkdirAll(screenshotDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create screenshots directory: %w", err)
		}

		// Generate filename if not provided
		filename := params.Filename
		if filename == "" {
			timestamp := time.Now().Format("2006-01-02_15-04-05")
			filename = fmt.Sprintf("screenshot_%s", timestamp)
		}

		// Add .png extension
		filePath := filepath.Join(screenshotDir, filename+".png")

		// Configure screenshot options
		opts := ScreenshotOptions{
			Interactive: params.Interactive,
			CameraNoise: params.CameraNoise,
			Delay:       params.Delay,
		}

		// Capture the screenshot
		resultPath, err := CaptureScreenshotWithOptions(filePath, opts)
		if err != nil {
			slog.Error("Screenshot capture failed",
				"error", err,
				"path", filePath,
				"interactive", params.Interactive,
				"delay", params.Delay)
			return &ScreenShotTool{
				Success: false,
				Message: fmt.Sprintf("Failed to capture screenshot: %v", err),
			}, nil
		}

		slog.Info("Screenshot captured successfully",
			"path", resultPath,
			"interactive", params.Interactive,
			"delay", params.Delay,
			"silent", !params.CameraNoise)

		return &ScreenShotTool{
			Success:  true,
			FilePath: resultPath,
			Message:  fmt.Sprintf("Screenshot saved to %s", resultPath),
		}, nil
	}

	return tools.NewTool(
		"ScreenshotTool",
		screenshotToolDescription,
		handler,
		tools.WithVerb("Capturing screenshot"),
		tools.WithLongRunning(false),
	)
}

// ScreenshotOptions configures the screenshot capture
type ScreenshotOptions struct {
	Interactive bool // -i flag: select area/window
	CameraNoise bool // Default false = silent. Set true to enable camera sound
	Delay       int  // -T flag: delay in seconds
}

// CaptureScreenshotWithOptions captures a screenshot with custom options
// By default, screenshots are silent (no camera sound)
func CaptureScreenshotWithOptions(filePath string, opts ScreenshotOptions) (string, error) {

	if !CanScreenshot() {
		slog.Warn("screenshot not supported on this OS")
		return "", nil
	}

	var args []string

	// -m captures the main display explicitly
	args = append(args, "-m")

	if !opts.CameraNoise {
		args = append(args, "-x")
	}

	if opts.Interactive {
		args = append(args, "-i")
	}

	if opts.Delay > 0 {
		args = append(args, "-T", fmt.Sprintf("%d", opts.Delay))
	}

	args = append(args, filePath)

	cmd := exec.Command("screencapture", args...)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	return filePath, nil
}

func CanScreenshot() bool {
	switch runtime.GOOS {
	case "darwin":
		return true
	default:
		return false
	}
}
