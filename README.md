# minimcp

[![Go Reference](https://pkg.go.dev/badge/github.com/mhpenta/minimcp.svg)](https://pkg.go.dev/github.com/mhpenta/minimcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Lightweight, type-safe MCP server implementation in Go with automatic schema generation and resilient JSON parsing.

## Features

- **MCP Tool Server** - Stdio and HTTP transports for serving tools via JSON-RPC 2.0
- **Type-Safe Tools** - Automatic schema generation from Go types using generics
- **Resilient JSON** - Strict parsing by default, opt-in repair for malformed input
- **Zero Config** - Create tools from plain Go functions with `tools.NewTool()`

## Core Packages

- **minimcp/mcp** - MCP server and transports (stdio/HTTP)
- **minimcp/tools** - Tool interface and TypedTool for type-safe tool creation
- **minimcp/infer** - Automatic JSON schema generation from Go types, using the new [google/jsonschema-go](https://github.com/google/jsonschema-go) package from the Go team.
- **minimcp/safeunmarshal** - Resilient JSON unmarshalling with size limits, with optional (but potentially dangerous) auto repair features

## Installation

```bash
go get github.com/mhpenta/minimcp
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/mhpenta/minimcp/mcp"
    "github.com/mhpenta/minimcp/tools"
)

// Define your tool's input/output types
type WeatherRequest struct {
    City string `json:"city"`
}

type WeatherResponse struct {
    Temperature float64 `json:"temperature"`
    Conditions  string  `json:"conditions"`
}

// Write a handler function
func getWeather(ctx context.Context, req WeatherRequest) (WeatherResponse, error) {
    return WeatherResponse{Temperature: 22.5, Conditions: "Sunny"}, nil
}

func main() {
    // Create tool from function - schema auto-generated
    weatherTool := tools.NewTool("get_weather", "Get current weather", getWeather)

    // Create and start server
    server := mcp.NewServer(mcp.ServerConfig{
        Name:    "weather-server",
        Version: "1.0.0",
        Tools:   []tools.Tool{weatherTool},
    })

    transport := mcp.NewStdioTransport(server, nil)
    transport.Start(context.Background())
}
```

## Creating Tools

### TypedTool (Recommended)

Use `tools.NewTool()` to create tools from handler functions. Schemas are automatically generated from your types:

```go
// Define types with jsonschema tags for rich descriptions
type CalculatorInput struct {
    Operation string  `json:"operation" jsonschema:"The arithmetic operation to perform (add, subtract, multiply, divide)"`
    A         float64 `json:"a" jsonschema:"The first operand in the calculation"`
    B         float64 `json:"b" jsonschema:"The second operand in the calculation"`
}

type CalculatorOutput struct {
    Result float64 `json:"result" jsonschema:"The computed result of the operation"`
}

// Handler function
func calculate(ctx context.Context, input CalculatorInput) (CalculatorOutput, error) {
    var result float64
    switch input.Operation {
    case "add":
        result = input.A + input.B
    case "multiply":
        result = input.A * input.B
    default:
        return CalculatorOutput{}, fmt.Errorf("unknown operation")
    }
    return CalculatorOutput{Result: result}, nil
}

// Create tool
tool := tools.NewTool("calculator", "Performs arithmetic operations", calculate)
```

**Tool Options:**
```go
tool := tools.NewTool(
    "my_tool",
    "Description",
    handler,
    tools.WithVerb("Processing"),       // UI verb for progress display
    tools.WithLongRunning(true),        // Hints this tool takes time
    tools.WithType("custom_type"),      // Custom type identifier
)
```

### Manual Tool Implementation

For full control, implement the `Tool` interface using `infer` and `safeunmarshal` directly:

```go
type MyTool struct{}

func (t *MyTool) Spec() *tools.ToolSpec {
    inputSchema, _ := infer.FromType[MyInput]()
    outputSchema, _ := infer.FromType[MyOutput]()
    inputMap, _ := infer.ToMap(inputSchema)
    outputMap, _ := infer.ToMap(outputSchema)

    return &tools.ToolSpec{
        Name:        "my_tool",
        Description: "Does something useful",
        Parameters:  inputMap,
        Output:      outputMap,
    }
}

func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
    input, err := safeunmarshal.To[MyInput](params)  // Strict by default
    if err != nil {
        return nil, err
    }

    output := processInput(input)
    return &tools.ToolResult{Output: output}, nil
}
```

## Package Details

### minimcp/safeunmarshal

Safe JSON unmarshalling with configurable strictness:

```go
import "github.com/mhpenta/minimcp/safeunmarshal"

// Strict mode (default) - only accepts well-formed JSON
config, err := safeunmarshal.To[Config](data)

// Lenient mode - attempts repair on malformed JSON, useful for marshaling non-critical LLM output from weak LLMs
config, err := safeunmarshal.ToLenient[Config](data)

// Custom options
config, err := safeunmarshal.ToWithOptions[Config](data, safeunmarshal.UnmarshalOptions{
    MaxInputSize: 1024 * 1024,  // 1MB limit (default 10MB)
    EnableRepair: true,          // Enable JSON repair
})
```

Features:
- **Strict by default** - Production-safe parsing
- **Optional repair** - Handle malformed JSON from LLMs (use `ToLenient()`)
- **Size limits** - Default 10MB max to prevent DoS
- **Text extraction** - Finds JSON embedded in text

### minimcp/infer

Automatic schema generation from Go types:

```go
import "github.com/mhpenta/minimcp/infer"

// From function signature
inputSchema, outputSchema, err := infer.FromFunc(myHandler)

// Convert to map for JSON encoding
schemaMap, err := infer.ToMap(schema)
```

### minimcp/mcp

MCP server with stdio and HTTP transports:

```go
// Stdio transport (for Claude Code or Claude Desktop)
server := mcp.NewServer(mcp.ServerConfig{
    Name:    "my-server",
    Version: "1.0.0",
    Tools:   []tools.Tool{myTool},
})
transport := mcp.NewStdioTransport(server, nil)
transport.Start(ctx)

// HTTP transport (for remote access)
validator := mcp.NewDEVKeyValidator()  // or implement APIKeyValidator
httpTransport := mcp.NewHTTPTransport(server, logger, validator)
httpTransport.Start(ctx, "8080")
```

## Examples

### SQL Server

A complete example demonstrating how to create an MCP server with a read-only SQL query tool for PostgreSQL:

```bash
cd examples/sql_server
go build -o sql-server main.go

# Run with your database credentials
./sql-server -dbname=mydb -user=postgres -password=secret
```

See [examples/sql_server/README.md](examples/sql_server/README.md) for detailed usage and configuration options.

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
