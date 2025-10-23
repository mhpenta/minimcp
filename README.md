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

### Using with jobj/funcschema

For automatic schema generation from Go functions, use [jobj's funcschema](https://github.com/mhpenta/jobj):

```go
import (
    "context"
    "encoding/json"

    "github.com/mhpenta/minimcp/tools"
    "github.com/mhpenta/jobj/funcschema"
)

// Define input/output types
type StockPriceInput struct {
    Symbol string `json:"symbol" jsonschema:"required,description=Stock ticker symbol"`
}

type StockPriceOutput struct {
    Price      float64 `json:"price"`
    MarketCap  float64 `json:"market_cap"`
}

type StockPriceTool struct{}

// Handler function with typed parameters
func (t *StockPriceTool) GetPrice(ctx context.Context, input StockPriceInput) (StockPriceOutput, error) {
    // Your implementation here
    return StockPriceOutput{Price: 150.50, MarketCap: 2.5e12}, nil
}

func (t *StockPriceTool) Spec() *tools.ToolSpec {
    // Generate schemas from handler function signature
    schemaIn, schemaOut, err := funcschema.SafeSchemasFromFunc(t.GetPrice)
    if err != nil {
        panic(err)
    }

    return &tools.ToolSpec{
        Name:        "get_stock_price",
        Description: "Fetches current stock price and market cap",
        Parameters:  schemaIn,  // Auto-generated input schema
        Output:      schemaOut, // Auto-generated output schema
    }
}

func (t *StockPriceTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
    var input StockPriceInput
    if err := json.Unmarshal(params, &input); err != nil {  // Or use safeunmarshal from the jobj package
        return nil, err
    }

    output, err := t.GetPrice(ctx, input)
    if err != nil {
        return nil, err
    }

    return &tools.ToolResult{
        Output: output,
    }, nil
}
```

This pattern:
- Automatically generates JSON schemas from Go types
- Provides type safety for inputs and outputs
- Uses struct tags for schema constraints
- No manual schema writing required

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
