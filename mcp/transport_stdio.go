package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
)

// StdioTransport provides stdio-based MCP server (reads from stdin, writes to stdout)
type StdioTransport struct {
	server         *Server
	logger         *slog.Logger
	jsonrpcHandler *JSONRPCHandler
	reader         io.Reader
	writer         io.Writer
}

// NewStdioTransport creates a stdio transport (no auth needed for local process)
func NewStdioTransport(server *Server, logger *slog.Logger) *StdioTransport {
	return &StdioTransport{
		server:         server,
		logger:         logger,
		jsonrpcHandler: NewJSONRPCHandler(server),
		reader:         os.Stdin,
		writer:         os.Stdout,
	}
}

// NewStdioTransportWithIO creates a stdio transport with custom reader/writer (for testing)
func NewStdioTransportWithIO(server *Server, logger *slog.Logger, reader io.Reader, writer io.Writer) *StdioTransport {
	return &StdioTransport{
		server:         server,
		logger:         logger,
		jsonrpcHandler: NewJSONRPCHandler(server),
		reader:         reader,
		writer:         writer,
	}
}

// Start begins reading from stdin and processing JSON-RPC messages
func (t *StdioTransport) Start(ctx context.Context) error {
	t.logger.Info("starting MCP stdio transport")

	scanner := bufio.NewScanner(t.reader)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max message size

	// Channel to receive scan results
	scanChan := make(chan []byte)
	errChan := make(chan error, 1)

	// Start scanner in goroutine
	go func() {
		defer close(scanChan)
		for scanner.Scan() {
			line := make([]byte, len(scanner.Bytes()))
			copy(line, scanner.Bytes())
			scanChan <- line
		}
		if err := scanner.Err(); err != nil {
			errChan <- err
		}
	}()

	for {
		select {
		case <-ctx.Done():
			t.logger.Info("stdio transport shutting down")
			return nil

		case line, ok := <-scanChan:
			if !ok {
				// Scanner closed
				select {
				case err := <-errChan:
					t.logger.Error("scanner error", "error", err)
					return err
				default:
					return nil
				}
			}

			if len(line) == 0 {
				continue
			}

			// Process the JSON-RPC message
			resp, err := t.jsonrpcHandler.HandleMessage(ctx, line)
			if err != nil {
				t.logger.Error("error handling message", "error", err)
				continue
			}

			// Write response if not a notification
			if resp != nil {
				respBytes, err := json.Marshal(resp)
				if err != nil {
					t.logger.Error("error marshaling response", "error", err)
					continue
				}

				// Write newline-delimited JSON to stdout
				if _, err := t.writer.Write(append(respBytes, '\n')); err != nil {
					t.logger.Error("error writing response", "error", err)
					return err
				}
			}
		}
	}
}
