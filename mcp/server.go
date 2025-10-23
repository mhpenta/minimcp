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
