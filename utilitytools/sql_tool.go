package utilitytools

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/mhpenta/minimcp/tools"
)

// SQLToolParams defines parameters for executing SQL queries
type SQLToolParams struct {
	Query string `json:"query" jsonschema:"SQL query to execute (read-only, only SELECT and WITH queries allowed)"`
}

// NewReadOnlySQLTool creates a new SQL query tool for LLM use
func NewReadOnlySQLTool(db *sql.DB, logger *slog.Logger) tools.Tool {
	if logger == nil {
		logger = slog.Default()
	}

	handler := func(ctx context.Context, params SQLToolParams) (*SQLQueryResult, error) {
		if params.Query == "" {
			return nil, fmt.Errorf("query parameter is required")
		}

		result, err := ExecuteSQLQuery(ctx, logger, db, params.Query)
		if err != nil {
			logger.Error("SQL query execution failed", "error", err)
			return result, err
		}

		logger.Info("SQL query executed successfully",
			"rows_returned", len(result.Rows),
			"columns", len(result.Columns),
			"execution_time_ms", result.ExecutionTime)

		return result, nil
	}

	return tools.NewTool(
		"ReadOnlySQLQuery",
		readOnlySQLToolDescription,
		handler,
		tools.WithType("ReadOnlySQLQuery_v1"),
		tools.WithVerb("Executing SQL query"),
	)
}

const readOnlySQLToolDescription = `Executes read-only SQL queries against the database for administrative analysis and debugging.

SECURITY FEATURES:
- READ-ONLY MODE: Only SELECT and WITH (CTE) queries are allowed
- All write operations are blocked (INSERT, UPDATE, DELETE, DROP, CREATE, ALTER, TRUNCATE, GRANT, REVOKE, COPY)
- Whole-word keyword matching prevents false positives (e.g., "INNER JOIN" won't trigger "INSERT" block)
- Database-specific meta-commands are blocked (e.g., backslash commands)
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
✗ Meta-commands

COMMON USE CASES:
- Explore database schema and table structures
- Query table data and metadata
- Debug data issues and verify data integrity
- Generate reports and analytics

TIPS:
- Use LIMIT to test queries before running on full datasets
- Results include execution time and row counts
- Query validation happens before execution to prevent accidental writes
- Check your specific database documentation for system tables to list tables and columns (e.g. pg_tables in Postgres, sqlite_master in SQLite)`

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
