// Package mcp provides a lightweight implementation of the Model Context Protocol (MCP).
//
// MCP enables communication between AI systems and tool providers. This implementation
// includes both stdio (for local tools like Claude Desktop) and HTTP (for remote services)
// transports, with full JSON-RPC 2.0 support.
//
// # Basic Usage
//
// Create and start a stdio server (for Claude Desktop):
//
//	import (
//	    "context"
//	    "github.com/mhpenta/minimcp/mcp"
//	    "github.com/mhpenta/minimcp/tools"
//	)
//
//	func main() {
//	    server := mcp.NewServer(mcp.ServerConfig{
//	        Name:    "my-server",
//	        Version: "1.0.0",
//	        Tools:   []tools.Tool{ /* your tools */ },
//	    })
//
//	    transport := mcp.NewStdioTransport(server, nil)
//	    transport.Start(context.Background())
//	}
//
// # HTTP Transport
//
// For remote access via HTTP:
//
//	validator := mcp.NewDEVKeyValidator() // or implement your own
//	httpTransport := mcp.NewHTTPTransport(server, logger, validator)
//	httpTransport.Start(ctx, "8080")
//
// # Protocol
//
// This implementation follows the MCP specification:
//   - JSON-RPC 2.0 for all communication
//   - Stdio transport for local servers
//   - HTTP transport with Bearer token authentication
//   - Standard MCP methods: initialize, tools/list, tools/call
//
// See https://modelcontextprotocol.io for full protocol documentation.
package mcp

import (
	"github.com/mhpenta/minimcp/tools"
	"log/slog"
)

// Server represents an MCP server that exposes tools
type Server struct {
	name    string
	version string
	tools   []tools.Tool
	logger  *slog.Logger
}

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	Name    string
	Version string
	Tools   []tools.Tool
	Logger  *slog.Logger
}

// NewServer creates a new MCP server with the provided tools
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	server := &Server{
		name:    cfg.Name,
		version: cfg.Version,
		tools:   cfg.Tools,
		logger:  cfg.Logger,
	}

	server.logger.Info("initialized MCP server",
		"name", cfg.Name,
		"version", cfg.Version,
		"tool_count", len(cfg.Tools))

	return server
}

// GetTools returns all registered tools
func (s *Server) GetTools() []tools.Tool {
	return s.tools
}

// Name returns the server name
func (s *Server) Name() string {
	return s.name
}

// Version returns the server version
func (s *Server) Version() string {
	return s.version
}
