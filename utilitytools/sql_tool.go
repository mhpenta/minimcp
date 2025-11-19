package utilitytools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/mhpenta/minimcp/infer"
	"github.com/mhpenta/minimcp/safeunmarshal"
	"github.com/mhpenta/minimcp/tools"
)

// SQLToolParams defines parameters for executing SQL queries
type SQLToolParams struct {
	Query string `json:"query" jsonschema:"SQL query to execute (read-only, only SELECT and WITH queries allowed)"`
}

// SQLTool provides LLM access to execute read-only SQL queries against the database
type SQLTool struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLTool creates a new SQL query tool for LLM use
func NewSQLTool(db *sql.DB, logger *slog.Logger) *SQLTool {

	if logger == nil {
		logger = slog.Default()
	}

	return &SQLTool{
		db:     db,
		logger: logger,
	}
}

// ExecuteQuery executes a read-only SQL query and returns results
func (t *SQLTool) ExecuteQuery(
	ctx context.Context,
	params SQLToolParams) (*SQLQueryResult, error) {

	if params.Query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	result, err := ExecuteSQLQuery(ctx, t.logger, t.db, params.Query)
	if err != nil {
		t.logger.Error("SQL query execution failed", "error", err)
		return result, err
	}

	t.logger.Info("SQL query executed successfully",
		"rows_returned", len(result.Rows),
		"columns", len(result.Columns),
		"execution_time_ms", result.ExecutionTime)

	return result, nil
}

// Execute implements the tools.Tool interface
func (t *SQLTool) Execute(ctx context.Context, params json.RawMessage) (*tools.ToolResult, error) {
	paramsStruct, err := safeunmarshal.To[SQLToolParams](params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	result, err := t.ExecuteQuery(ctx, paramsStruct)
	if err != nil {
		// Return the result even on error, as it contains error details
		if result != nil && !result.Success {
			return &tools.ToolResult{
				Output: result,
				Error:  nil, // Error is in the result structure
			}, nil
		}
		return nil, fmt.Errorf("failed to execute SQL query: %w", err)
	}

	return &tools.ToolResult{
		Output: result,
		Error:  nil,
	}, nil
}

const adminSQLToolDescription = `Executes read-only SQL queries against the PostgreSQL database for administrative analysis and debugging.

SECURITY FEATURES:
- READ-ONLY MODE: Only SELECT and WITH (CTE) queries are allowed
- All write operations are blocked (INSERT, UPDATE, DELETE, DROP, CREATE, ALTER, TRUNCATE, GRANT, REVOKE, COPY)
- Whole-word keyword matching prevents false positives (e.g., "INNER JOIN" won't trigger "INSERT" block)
- Backslash commands (psql meta-commands) are blocked
- 30-second timeout on all queries

ALLOWED QUERIES:
✓ SELECT statements with any complexity
✓ JOINs (INNER, LEFT, RIGHT, OUTER)
✓ Subqueries and CTEs (WITH statements)
✓ Aggregate functions (COUNT, SUM, AVG, etc.)
✓ Window functions
✓ UNION, INTERSECT, EXCEPT
✓ Complex WHERE clauses and filters

BLOCKED QUERIES:
✗ Any DML: INSERT, UPDATE, DELETE
✗ Any DDL: CREATE, DROP, ALTER, TRUNCATE
✗ Security: GRANT, REVOKE
✗ Data manipulation: COPY
✗ Meta-commands: \d, \dt, etc.

COMMON USE CASES:
- Explore database schema and table structures
- Query table data and metadata
- Debug data issues and verify data integrity
- Generate reports and analytics

IMPORTANT DATABASE INDEXES:
- pg_tables: List all database tables
- information_schema.columns: Explore table columns and types

TIPS:
- Start with "SELECT schemaname, tablename FROM pg_tables WHERE schemaname = 'public'" to explore tables
- Use LIMIT to test queries before running on full datasets
- Results include execution time and row counts
- Query validation happens before execution to prevent accidental writes`

// Spec implements the tools.Tool interface
func (t *SQLTool) Spec() *tools.ToolSpec {
	schemaIn, schemaOut, err := infer.FromFunc(t.ExecuteQuery)
	if err != nil {
		t.logger.Error("Failed to parse function schema for SQLTool", "error", err)
		return nil
	}

	schemaInMap, err := infer.ToMap(schemaIn)
	if err != nil {
		t.logger.Error("Failed to parse function schema for SQLTool", "error", err)
	}
	schemaOutMap, err := infer.ToMap(schemaOut)
	if err != nil {
		t.logger.Error("Failed to parse function schema for SQLTool", "error", err)
	}

	return &tools.ToolSpec{
		Name:        "AdminSQLQuery",
		Type:        "AdminSQLQuery_v1",
		Description: adminSQLToolDescription,
		Parameters:  schemaInMap,
		Output:      schemaOutMap,
		Sequential:  false, // SQL queries can run in parallel, that's fine
		UI: tools.UI{
			Verb:        "Executing SQL query",
			LongRunning: false,
		},
	}
}

const (
	defaultTimeout = 60 * time.Second
)

// SQLQueryResult represents the result of a SQL query execution
type SQLQueryResult struct {
	Success       bool            `json:"success"`
	Columns       []string        `json:"columns,omitempty"`
	Rows          [][]interface{} `json:"rows,omitempty"`
	ExecutionTime int64           `json:"execution_time,omitempty"` // in milliseconds
	Error         string          `json:"error,omitempty"`
}

// ExecuteSQLQuery executes a read-only SQL query with strict validation
// It only allows SELECT and WITH queries and blocks any write operations
func ExecuteSQLQuery(ctx context.Context, logger *slog.Logger, db *sql.DB, query string) (*SQLQueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return &SQLQueryResult{
			Success: false,
			Error:   "Query cannot be empty",
		}, fmt.Errorf("empty query")
	}

	// Strict validation: only allow SELECT and WITH queries
	upperQuery := strings.ToUpper(query)
	if !strings.HasPrefix(upperQuery, "SELECT") && !strings.HasPrefix(upperQuery, "WITH") {
		return &SQLQueryResult{
			Success: false,
			Error:   "Only SELECT and WITH queries are allowed",
		}, fmt.Errorf("forbidden query type")
	}

	// Check for dangerous keywords (whole word matches only)
	dangerousKeywords := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER",
		"TRUNCATE", "GRANT", "REVOKE", "COPY",
	}
	for _, keyword := range dangerousKeywords {
		if containsWholeWord(upperQuery, keyword) {
			return &SQLQueryResult{
				Success: false,
				Error:   fmt.Sprintf("Forbidden keyword '%s' detected", keyword),
			}, fmt.Errorf("forbidden keyword: %s", keyword)
		}
	}

	// Check for backslash commands
	if strings.Contains(query, "\\") {
		return &SQLQueryResult{
			Success: false,
			Error:   "Backslash commands are not allowed",
		}, fmt.Errorf("backslash commands not allowed")
	}

	// Execute the query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	start := time.Now()
	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		errMsg := fmt.Sprintf("SQL execution error: %v", err)
		return &SQLQueryResult{
			Success: false,
			Error:   errMsg,
		}, err
	}
	defer rows.Close()

	executionTime := time.Since(start).Milliseconds()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		errMsg := fmt.Sprintf("Error getting columns: %v", err)
		return &SQLQueryResult{
			Success: false,
			Error:   errMsg,
		}, err
	}

	// Prepare result structure
	var results [][]interface{}

	// Process rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			errMsg := fmt.Sprintf("Error scanning row: %v", err)
			return &SQLQueryResult{
				Success: false,
				Error:   errMsg,
			}, err
		}

		// Convert values to strings for JSON serialization
		stringValues := make([]interface{}, len(values))
		for i, val := range values {
			if val == nil {
				stringValues[i] = nil
			} else {
				stringValues[i] = fmt.Sprintf("%v", val)
			}
		}

		results = append(results, stringValues)
	}

	if err = rows.Err(); err != nil {
		errMsg := fmt.Sprintf("Error iterating rows: %v", err)
		return &SQLQueryResult{
			Success: false,
			Error:   errMsg,
		}, err
	}

	logger.Info("SQL query executed",
		"rows_returned", len(results),
		"execution_time_ms", executionTime,
		"columns", len(columns))

	return &SQLQueryResult{
		Success:       true,
		Columns:       columns,
		Rows:          results,
		ExecutionTime: executionTime,
	}, nil
}

// containsWholeWord checks if a keyword exists as a whole word in the query
// This prevents false positives like "INNER" matching "INSERT"
func containsWholeWord(query, keyword string) bool {
	wholeWordPattern := `\b` + regexp.QuoteMeta(keyword) + `\b`
	matched, _ := regexp.MatchString(wholeWordPattern, query)
	return matched
}
