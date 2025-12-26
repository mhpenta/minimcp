package utilitytools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/mhpenta/minimcp/tools"
)

// ElasticToolParams defines parameters for Elasticsearch operations
type ElasticToolParams struct {
	Operation string `json:"operation" jsonschema:"Operation to perform: list_indices, get_mapping, or query"`
	Index     string `json:"index,omitempty" jsonschema:"Index name (required for get_mapping and query operations)"`
	Query     string `json:"query,omitempty" jsonschema:"Elasticsearch query in JSON format (required for query operation)"`
}

// NewElasticTool creates a new general-purpose Elasticsearch tool for LLM use
func NewElasticTool(esClient *elasticsearch.Client, logger *slog.Logger) tools.Tool {
	if logger == nil {
		logger = slog.Default()
	}

	handler := func(ctx context.Context, params ElasticToolParams) (*ElasticResult, error) {
		switch strings.ToLower(params.Operation) {
		case "list_indices":
			return listIndices(ctx, esClient, logger)
		case "get_mapping":
			if params.Index == "" {
				return nil, fmt.Errorf("index parameter is required for get_mapping operation")
			}
			return getMapping(ctx, esClient, logger, params.Index)
		case "query":
			if params.Index == "" {
				return nil, fmt.Errorf("index parameter is required for query operation")
			}
			if params.Query == "" {
				return nil, fmt.Errorf("query parameter is required for query operation")
			}
			return executeQuery(ctx, esClient, logger, params.Index, params.Query)
		default:
			return nil, fmt.Errorf("invalid operation: %s (must be list_indices, get_mapping, or query)", params.Operation)
		}
	}

	return tools.NewTool(
		"Elasticsearch",
		elasticToolDescription,
		handler,
		tools.WithVerb("Querying Elasticsearch"),
	)
}

// ElasticResult represents the result of any Elasticsearch operation
type ElasticResult struct {
	Success   bool                   `json:"success"`
	Operation string                 `json:"operation"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// listIndices returns all available indices with basic stats
func listIndices(ctx context.Context, esClient *elasticsearch.Client, logger *slog.Logger) (*ElasticResult, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := esClient.Cat.Indices(
		esClient.Cat.Indices.WithContext(queryCtx),
		esClient.Cat.Indices.WithFormat("json"),
		esClient.Cat.Indices.WithH("index", "docs.count", "store.size", "health", "status"),
	)
	if err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "list_indices",
			Error:     fmt.Sprintf("Failed to list indices: %v", err),
		}, err
	}
	defer res.Body.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "list_indices",
			Error:     fmt.Sprintf("Failed to read response: %v", err),
		}, err
	}

	var indices []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &indices); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "list_indices",
			Error:     fmt.Sprintf("Failed to parse response: %v", err),
		}, err
	}

	// Filter out system indices (those starting with .)
	var userIndices []map[string]interface{}
	for _, idx := range indices {
		if name, ok := idx["index"].(string); ok && !strings.HasPrefix(name, ".") {
			userIndices = append(userIndices, idx)
		}
	}

	logger.Info("Listed Elasticsearch indices", "count", len(userIndices))

	return &ElasticResult{
		Success:   true,
		Operation: "list_indices",
		Data: map[string]interface{}{
			"indices": userIndices,
			"count":   len(userIndices),
		},
	}, nil
}

// getMapping returns the mapping (schema) for a specific index
func getMapping(ctx context.Context, esClient *elasticsearch.Client, logger *slog.Logger, index string) (*ElasticResult, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := esClient.Indices.GetMapping(
		esClient.Indices.GetMapping.WithContext(queryCtx),
		esClient.Indices.GetMapping.WithIndex(index),
	)
	if err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "get_mapping",
			Error:     fmt.Sprintf("Failed to get mapping: %v", err),
		}, err
	}
	defer res.Body.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "get_mapping",
			Error:     fmt.Sprintf("Failed to read response: %v", err),
		}, err
	}

	var mapping map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &mapping); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "get_mapping",
			Error:     fmt.Sprintf("Failed to parse response: %v", err),
		}, err
	}

	// Check for Elasticsearch error
	if errorInfo, exists := mapping["error"]; exists {
		errorJSON, _ := json.Marshal(errorInfo)
		return &ElasticResult{
			Success:   false,
			Operation: "get_mapping",
			Error:     fmt.Sprintf("Elasticsearch error: %s", string(errorJSON)),
		}, fmt.Errorf("elasticsearch error: %s", string(errorJSON))
	}

	logger.Info("Retrieved index mapping", "index", index)

	return &ElasticResult{
		Success:   true,
		Operation: "get_mapping",
		Data:      mapping,
	}, nil
}

// executeQuery executes an Elasticsearch query and returns results
func executeQuery(ctx context.Context, esClient *elasticsearch.Client, logger *slog.Logger, index string, query string) (*ElasticResult, error) {
	// Validate the query is valid JSON
	var queryObj interface{}
	if err := json.Unmarshal([]byte(query), &queryObj); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "query",
			Error:     fmt.Sprintf("Invalid JSON query: %v", err),
		}, fmt.Errorf("invalid JSON: %w", err)
	}

	// Execute the query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := esClient.Search(
		esClient.Search.WithContext(queryCtx),
		esClient.Search.WithIndex(index),
		esClient.Search.WithBody(strings.NewReader(query)),
		esClient.Search.WithPretty(),
	)
	if err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "query",
			Error:     fmt.Sprintf("Failed to execute query: %v", err),
		}, err
	}
	defer res.Body.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "query",
			Error:     fmt.Sprintf("Failed to read response: %v", err),
		}, err
	}

	var esResponse map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &esResponse); err != nil {
		return &ElasticResult{
			Success:   false,
			Operation: "query",
			Error:     fmt.Sprintf("Failed to parse response: %v", err),
		}, err
	}

	// Check for Elasticsearch error
	if errorInfo, exists := esResponse["error"]; exists {
		errorJSON, _ := json.Marshal(errorInfo)
		return &ElasticResult{
			Success:   false,
			Operation: "query",
			Error:     fmt.Sprintf("Elasticsearch error: %s", string(errorJSON)),
		}, fmt.Errorf("elasticsearch error: %s", string(errorJSON))
	}

	// Extract stats for logging
	hits := int64(0)
	took := int64(0)
	if hitsInfo, ok := esResponse["hits"].(map[string]interface{}); ok {
		if total, ok := hitsInfo["total"].(map[string]interface{}); ok {
			if value, ok := total["value"].(float64); ok {
				hits = int64(value)
			}
		}
	}
	if tookValue, ok := esResponse["took"].(float64); ok {
		took = int64(tookValue)
	}

	logger.Info("Elasticsearch query executed",
		"index", index,
		"hits", hits,
		"took_ms", took)

	return &ElasticResult{
		Success:   true,
		Operation: "query",
		Data:      esResponse,
	}, nil
}

const elasticToolDescription = `Executes Elasticsearch operations for searching and analyzing indexed data.

OPERATIONS:
This tool supports three operations via the "operation" parameter:

1. list_indices - Discover available indices
   Returns all user indices with document count, size, health, and status.
   No additional parameters required.

2. get_mapping - Get index schema
   Returns the field mapping (schema) for a specific index.
   Requires: index parameter

3. query - Execute search queries
   Runs an Elasticsearch Query DSL search.
   Requires: index and query parameters

RECOMMENDED WORKFLOW:
1. First use list_indices to discover available indices
2. Use get_mapping on relevant indices to understand their fields
3. Build and execute queries based on the discovered schema

QUERY DSL BASICS:
Queries use JSON format with the Elasticsearch Query DSL:

{
  "query": {
    "bool": {
      "must": [
        {"term": {"field_name": "exact_value"}},
        {"match": {"text_field": "search terms"}}
      ],
      "filter": [
        {"range": {"date_field": {"gte": "2024-01-01"}}}
      ]
    }
  },
  "size": 10,
  "sort": [{"date_field": {"order": "desc"}}]
}

QUERY TYPES:
- term: Exact match on keyword fields
- match: Full-text search on text fields
- match_phrase: Exact phrase matching
- range: Numeric/date range queries
- bool: Combine queries with must/should/must_not/filter
- query_string: Lucene query syntax

AGGREGATIONS:
{
  "size": 0,
  "aggs": {
    "by_category": {
      "terms": {"field": "category_field"}
    }
  }
}

RESPONSE FORMAT:
- list_indices: Returns {indices: [...], count: N}
- get_mapping: Returns index mapping with field types
- query: Returns full Elasticsearch response with hits and aggregations

TIPS:
- Use "size" to limit results (default varies by cluster)
- Use "from" for pagination
- Add "_source": ["field1", "field2"] to limit returned fields
- Queries timeout after 30 seconds
- System indices (starting with .) are filtered from list_indices`