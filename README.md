# minimcp

[![Go Reference](https://pkg.go.dev/badge/github.com/mhpenta/minimcp.svg)](https://pkg.go.dev/github.com/mhpenta/minimcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Zero dependency, permissive mini-MCP server implementation in Go.

Implements basic [Model Context Protocol (MCP)](https://modelcontextprotocol.io) functionality. Assumes tool creators build accurate JSON schemas, for instance with the funcschema package of [mhpenta/jobj](https://github.com/mhpenta/jobj). Does not enforce schema validation.

## Features

- **Stdio transport** - For local MCP servers (Claude Desktop, etc.)
- **HTTP transport** - With Bearer/API key authentication
- **JSON-RPC 2.0** - Full protocol handler
- **Tool execution** - Register and execute tools with proper schemas

## Installation

```bash
go get github.com/mhpenta/minimcp
```

## Quick Start

### Stdio Transport (for Claude Desktop)

```go
package main

import (
    "context"
    "log/slog"

    "github.com/mhpenta/minimcp/mcp"
    "github.com/mhpenta/minimcp/tools"
)

func main() {
    logger := slog.Default()

    server := mcp.NewServer(mcp.ServerConfig{
        Name:    "my-server",
        Version: "1.0.0",
        Tools:   []tools.Tool{/* your tools */},
        Logger:  logger,
    })

    transport := mcp.NewStdioTransport(server, logger)
    transport.Start(context.Background())
}
```

### HTTP Transport

```go
validator := mcp.NewDEVKeyValidator() // or implement your own
httpTransport := mcp.NewHTTPTransport(server, logger, validator)
httpTransport.Start(ctx, "8080")
```

## Creating Tools

Implement the `tools.Tool` interface:

```go
type MyTool struct{}

func (t *MyTool) Spec() *tools.ToolSpec {
    return &tools.ToolSpec{
        Name:        "my_tool",
        Description: "Does something useful",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "input": map[string]interface{}{
                    "type": "string",
                },
            },
        },
    }
}

func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
    // Your tool logic here
    return &tools.ToolResult{
        Output: "result",
    }, nil
}
```

## Testing

```bash
# Run tests
go test ./...

# Check coverage
go test ./... -cover

# View coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## License

MIT License - see [LICENSE](LICENSE) file for details
