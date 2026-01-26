# minimcp

[![Go Reference](https://pkg.go.dev/badge/github.com/mhpenta/minimcp.svg)](https://pkg.go.dev/github.com/mhpenta/minimcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Lightweight MCP server in Go. Designed for quick dev integration, not for production. 

**Features:** Stdio/HTTP transports • Type-safe tools • Auto schema generation • Resilient JSON parsing • Response size limits

**Packages:**
- `minimcp/mcp` - Server and transports (stdio/HTTP)
- `minimcp/tools` - Type-safe tool creation
- `minimcp/infer` - Schema generation via [google/jsonschema-go](https://github.com/google/jsonschema-go)
- `minimcp/safeunmarshal` - Safe JSON parsing with optional repair
- `minimcp/utilitytools` - Dev utilities (SQL, screenshots, Elastic, GCS)

**Requirements:** Go 1.23.0+

## Installation

```bash
go get github.com/mhpenta/minimcp
```

## Quick Start

```go
type Input struct { City string `json:"city"` }
type Output struct { Temp float64 `json:"temp"` }

func getWeather(ctx context.Context, in Input) (Output, error) {
    return Output{Temp: 22.5}, nil
}

func main() {
    tool := tools.NewTool("weather", "Get weather", getWeather)
    server := mcp.NewServer(mcp.ServerConfig{
        Name: "weather-server", Version: "1.0.0", Tools: []tools.Tool{tool},
    })
    mcp.NewStdioTransport(server, nil).Start(context.Background())
}
```

## Creating Tools

Use `tools.NewTool()` - schemas auto-generated from types:

```go
type CalcInput struct {
    Op string  `json:"op" jsonschema:"Operation: add, multiply"`
    A  float64 `json:"a"`
    B  float64 `json:"b"`
}

func calc(ctx context.Context, in CalcInput) (float64, error) {
    if in.Op == "add" { return in.A + in.B, nil }
    return in.A * in.B, nil
}

tool := tools.NewTool("calc", "Calculator", calc,
    tools.WithVerb("Computing"),    // UI progress verb
    tools.WithLongRunning(true),    // Hints tool takes time
)
```

**Manual implementation** (for full control, implement `Tool` interface):

```go
func (t *MyTool) Spec() *tools.ToolSpec {
    inputSchema, outputSchema, _ := infer.FromFunc(myHandler)
    inputMap, _ := infer.ToMap(inputSchema)
    return &tools.ToolSpec{Name: "my_tool", Description: "...", Parameters: inputMap}
}

func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
    input, err := safeunmarshal.To[MyInput](params)
    if err != nil { return nil, err }
    return &tools.ToolResult{Output: processInput(input)}, nil
}
```

## Configuration

**Response Size Limits:**
```go
server := mcp.NewServer(mcp.ServerConfig{
    MaxResponseBytes: 50000,  // Truncate at 50KB (0 = no limit)
})
// Truncated responses include: [TRUNCATED: original_size=X bytes, showing=Y bytes, truncated=Z bytes]
```

**JSON Parsing:**
```go
config, err := safeunmarshal.To[Config](data)              // Strict (default)
config, err := safeunmarshal.ToLenient[Config](data)       // Repair malformed JSON
config, err := safeunmarshal.ToWithOptions[Config](data, safeunmarshal.UnmarshalOptions{
    MaxInputSize: 1024 * 1024,  // 1MB limit (default 10MB)
    EnableRepair: true,
})
```

**Transports:**
```go
// Stdio (for Claude Code/Desktop)
mcp.NewStdioTransport(server, nil).Start(ctx)

// HTTP (remote access)
validator := mcp.NewDEVKeyValidator()  // Dev only - implement APIKeyValidator for prod
mcp.NewHTTPTransport(server, logger, validator).Start(ctx, "8080")
```

## Security

⚠️ `DEVKeyValidator` uses hardcoded key `please-change-me-dev-key` - **dev only**.

**Production:** Implement `APIKeyValidator` interface:
```go
func (v *ProdValidator) Validate(ctx context.Context, key string) bool {
    return secureCompare(key, expectedKey)  // Constant-time comparison
}
```

Best practices: HTTPS • Secure key storage • Rate limiting • Key rotation
Best practices: Don't use minimcp in production

## Testing

```bash
go test ./...                                        # Run tests
go test ./... -coverprofile=coverage.out             # Coverage report
go tool cover -html=coverage.out                     # View coverage
```
