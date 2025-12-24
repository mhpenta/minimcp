package utilitytools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/mhpenta/minimcp/tools"
	"google.golang.org/api/iterator"
)

// GCSToolParams defines parameters for GCS operations
type GCSToolParams struct {
	Operation string `json:"operation" jsonschema:"Operation to perform: 'list', 'download', 'read', or 'info'" required:"true"`
	URL       string `json:"url" jsonschema:"GCS URL (gs://bucket/path) or base path" required:"false"`
	Bucket    string `json:"bucket" jsonschema:"GCS bucket name (alternative to URL)" required:"false"`
	Path      string `json:"path" jsonschema:"Path/prefix within bucket (alternative to URL)" required:"false"`
	LocalPath string `json:"localPath" jsonschema:"Local path for download operation" required:"false"`
	Recursive bool   `json:"recursive" jsonschema:"List recursively (default: false)" required:"false"`
	MaxResults int   `json:"maxResults" jsonschema:"Maximum number of results to return (default: 100)" required:"false"`
}

// GCSToolResult represents the result of a GCS operation
type GCSToolResult struct {
	Success   bool           `json:"success"`
	Operation string         `json:"operation"`
	Message   string         `json:"message,omitempty"`
	Error     string         `json:"error,omitempty"`
	Objects   []GCSObjectInfo `json:"objects,omitempty"`
	LocalPath string         `json:"localPath,omitempty"`
	Content   string         `json:"content,omitempty"`
}

// GCSObjectInfo represents information about a GCS object
type GCSObjectInfo struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"contentType"`
	Updated      time.Time `json:"updated"`
	IsPrefix     bool      `json:"isPrefix,omitempty"`
	MD5Hash      string    `json:"md5Hash,omitempty"`
	CRC32C       uint32    `json:"crc32c,omitempty"`
	StorageClass string    `json:"storageClass,omitempty"`
}

const gcsToolDescription = `Provides READ-ONLY access to Google Cloud Storage (GCS) for listing, downloading, and streaming files.

This tool allows you to safely explore and retrieve data from GCS buckets without any ability to
modify, upload, or delete objects. All operations are strictly read-only for security.

SECURITY FEATURES:
- READ-ONLY MODE: Only read operations are allowed (list, download, stream, info)
- No write operations: Cannot upload, delete, modify, or create objects
- No bucket modifications: Cannot create, delete, or modify bucket settings
- Safe exploration: Browse and retrieve data without risk of accidental changes

OPERATIONS:
1. list - List objects in a GCS bucket or prefix
2. download - Download an object to local filesystem
3. read - Read object content into memory (returns as string, suitable for text files)
4. info - Get metadata about a specific object

PARAMETERS:
- operation (required): The operation to perform ('list', 'download', 'read', or 'info')
- url (optional): Full GCS URL (e.g., 'gs://bucket-name/path/to/file')
- bucket (optional): Bucket name (alternative to URL)
- path (optional): Path/prefix within bucket (alternative to URL, use '' for bucket root)
- localPath (optional): Local file path for download operation
- recursive (optional): For list operation, list all objects recursively (default: false)
- maxResults (optional): Maximum number of objects to return for list operation (default: 100)

URL FORMATS:
- Full URL: gs://bucket-name/path/to/object
- Or separate: bucket='bucket-name', path='path/to/object'

EXAMPLES:
1. List objects in a bucket:
   {"operation": "list", "url": "gs://my-bucket/data/"}

2. List all objects recursively:
   {"operation": "list", "bucket": "my-bucket", "path": "logs/", "recursive": true}

3. Download a file:
   {"operation": "download", "url": "gs://my-bucket/file.txt", "localPath": "./downloads/file.txt"}

4. Read a text file:
   {"operation": "read", "bucket": "my-bucket", "path": "config.json"}

5. Get object info:
   {"operation": "info", "url": "gs://my-bucket/data/file.csv"}

AUTHENTICATION:
- Uses Application Default Credentials (ADC)
- Set GOOGLE_APPLICATION_CREDENTIALS environment variable or use gcloud auth application-default login

RETURNS:
- success: Whether the operation succeeded
- operation: The operation that was performed
- message: Status message
- objects: List of objects (for 'list' operation)
- localPath: Path to downloaded file (for 'download' operation)
- content: File content (for 'read' operation)
- error: Error message if operation failed`

// NewReadOnlyGCSTool creates a new read-only GCS tool using the NewTool pattern
func NewReadOnlyGCSTool(ctx context.Context, logger *slog.Logger, downloadDir string) tools.Tool {
	if logger == nil {
		logger = slog.Default()
	}

	if downloadDir == "" {
		downloadDir = "./gcs_downloads"
	}

	handler := func(ctx context.Context, params GCSToolParams) (*GCSToolResult, error) {
		// Validate operation
		validOps := map[string]bool{"list": true, "download": true, "read": true, "info": true}
		if !validOps[params.Operation] {
			return &GCSToolResult{
				Success: false,
				Error:   "Invalid operation. Must be 'list', 'download', 'read', or 'info'",
			}, fmt.Errorf("invalid operation: %s", params.Operation)
		}

		// Parse URL or use bucket/path
		bucket, path, err := parseGCSLocation(params)
		if err != nil {
			return &GCSToolResult{
				Success:   false,
				Operation: params.Operation,
				Error:     err.Error(),
			}, err
		}

		// Create GCS client
		client, err := storage.NewClient(ctx)
		if err != nil {
			logger.Error("Failed to create GCS client", "error", err)
			return &GCSToolResult{
				Success:   false,
				Operation: params.Operation,
				Error:     fmt.Sprintf("Failed to create GCS client: %v", err),
			}, err
		}
		defer client.Close()

		// Execute operation
		switch params.Operation {
		case "list":
			return handleListOperation(ctx, logger, client, bucket, path, params)
		case "download":
			return handleDownloadOperation(ctx, logger, client, bucket, path, params, downloadDir)
		case "read":
			return handleReadOperation(ctx, logger, client, bucket, path)
		case "info":
			return handleInfoOperation(ctx, logger, client, bucket, path)
		default:
			return &GCSToolResult{
				Success:   false,
				Operation: params.Operation,
				Error:     "Unknown operation",
			}, fmt.Errorf("unknown operation: %s", params.Operation)
		}
	}

	return tools.NewTool(
		"ReadOnlyGCSTool",
		gcsToolDescription,
		handler,
		tools.WithVerb("Reading from GCS"),
		tools.WithLongRunning(false),
	)
}

// parseGCSLocation extracts bucket and path from params
func parseGCSLocation(params GCSToolParams) (bucket, path string, err error) {
	if params.URL != "" {
		// Parse gs://bucket/path format
		if !strings.HasPrefix(params.URL, "gs://") {
			return "", "", fmt.Errorf("invalid GCS URL format, must start with gs://")
		}

		trimmed := strings.TrimPrefix(params.URL, "gs://")
		parts := strings.SplitN(trimmed, "/", 2)

		bucket = parts[0]
		if len(parts) > 1 {
			path = parts[1]
		}
	} else if params.Bucket != "" {
		bucket = params.Bucket
		path = params.Path
	} else {
		return "", "", fmt.Errorf("must provide either 'url' or 'bucket' parameter")
	}

	if bucket == "" {
		return "", "", fmt.Errorf("bucket name cannot be empty")
	}

	return bucket, path, nil
}

// handleListOperation lists objects in a GCS bucket/prefix
func handleListOperation(ctx context.Context, logger *slog.Logger, client *storage.Client,
	bucket, prefix string, params GCSToolParams) (*GCSToolResult, error) {

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	query := &storage.Query{
		Prefix: prefix,
	}

	if !params.Recursive && prefix != "" {
		// Only list objects at this level (simulate directory listing)
		query.Delimiter = "/"
	}

	it := client.Bucket(bucket).Objects(ctx, query)

	var objects []GCSObjectInfo
	count := 0

	for count < maxResults {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			logger.Error("Error listing objects", "error", err, "bucket", bucket, "prefix", prefix)
			return &GCSToolResult{
				Success:   false,
				Operation: "list",
				Error:     fmt.Sprintf("Error listing objects: %v", err),
			}, err
		}

		// Handle both objects and prefixes (directories)
		if attrs.Prefix != "" {
			// This is a "directory" prefix
			objects = append(objects, GCSObjectInfo{
				Name:     attrs.Prefix,
				IsPrefix: true,
			})
		} else {
			// This is an actual object
			objects = append(objects, GCSObjectInfo{
				Name:         attrs.Name,
				Size:         attrs.Size,
				ContentType:  attrs.ContentType,
				Updated:      attrs.Updated,
				MD5Hash:      fmt.Sprintf("%x", attrs.MD5),
				CRC32C:       attrs.CRC32C,
				StorageClass: attrs.StorageClass,
				IsPrefix:     false,
			})
		}
		count++
	}

	logger.Info("Listed GCS objects",
		"bucket", bucket,
		"prefix", prefix,
		"count", len(objects),
		"recursive", params.Recursive)

	message := fmt.Sprintf("Found %d objects in gs://%s/%s", len(objects), bucket, prefix)
	if len(objects) >= maxResults {
		message += fmt.Sprintf(" (limited to %d results)", maxResults)
	}

	return &GCSToolResult{
		Success:   true,
		Operation: "list",
		Message:   message,
		Objects:   objects,
	}, nil
}

// handleDownloadOperation downloads an object from GCS to local filesystem
func handleDownloadOperation(ctx context.Context, logger *slog.Logger, client *storage.Client,
	bucket, objectPath string, params GCSToolParams, downloadDir string) (*GCSToolResult, error) {

	if objectPath == "" {
		return &GCSToolResult{
			Success:   false,
			Operation: "download",
			Error:     "Object path cannot be empty for download operation",
		}, fmt.Errorf("empty object path")
	}

	// Determine local file path
	localPath := params.LocalPath
	if localPath == "" {
		// Create download directory if it doesn't exist
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			return &GCSToolResult{
				Success:   false,
				Operation: "download",
				Error:     fmt.Sprintf("Failed to create download directory: %v", err),
			}, err
		}

		// Use the object's base name
		filename := filepath.Base(objectPath)
		localPath = filepath.Join(downloadDir, filename)
	} else {
		// Ensure parent directory exists
		dir := filepath.Dir(localPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &GCSToolResult{
				Success:   false,
				Operation: "download",
				Error:     fmt.Sprintf("Failed to create directory: %v", err),
			}, err
		}
	}

	// Create the local file
	file, err := os.Create(localPath)
	if err != nil {
		logger.Error("Failed to create local file", "error", err, "path", localPath)
		return &GCSToolResult{
			Success:   false,
			Operation: "download",
			Error:     fmt.Sprintf("Failed to create local file: %v", err),
		}, err
	}
	defer file.Close()

	// Download from GCS
	obj := client.Bucket(bucket).Object(objectPath)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		logger.Error("Failed to create GCS reader", "error", err, "bucket", bucket, "object", objectPath)
		return &GCSToolResult{
			Success:   false,
			Operation: "download",
			Error:     fmt.Sprintf("Failed to read from GCS: %v", err),
		}, err
	}
	defer reader.Close()

	// Copy data
	bytesWritten, err := io.Copy(file, reader)
	if err != nil {
		logger.Error("Failed to download file", "error", err)
		return &GCSToolResult{
			Success:   false,
			Operation: "download",
			Error:     fmt.Sprintf("Failed to download file: %v", err),
		}, err
	}

	logger.Info("Downloaded GCS object",
		"bucket", bucket,
		"object", objectPath,
		"localPath", localPath,
		"bytes", bytesWritten)

	return &GCSToolResult{
		Success:   true,
		Operation: "download",
		Message:   fmt.Sprintf("Downloaded gs://%s/%s to %s (%d bytes)", bucket, objectPath, localPath, bytesWritten),
		LocalPath: localPath,
	}, nil
}

// handleReadOperation reads an object's content from GCS into memory
func handleReadOperation(ctx context.Context, logger *slog.Logger, client *storage.Client,
	bucket, objectPath string) (*GCSToolResult, error) {

	if objectPath == "" {
		return &GCSToolResult{
			Success:   false,
			Operation: "read",
			Error:     "Object path cannot be empty for read operation",
		}, fmt.Errorf("empty object path")
	}

	obj := client.Bucket(bucket).Object(objectPath)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		logger.Error("Failed to create GCS reader", "error", err, "bucket", bucket, "object", objectPath)
		return &GCSToolResult{
			Success:   false,
			Operation: "read",
			Error:     fmt.Sprintf("Failed to read from GCS: %v", err),
		}, err
	}
	defer reader.Close()

	// Read content into memory
	content, err := io.ReadAll(reader)
	if err != nil {
		logger.Error("Failed to read content", "error", err)
		return &GCSToolResult{
			Success:   false,
			Operation: "read",
			Error:     fmt.Sprintf("Failed to read content: %v", err),
		}, err
	}

	logger.Info("Read GCS object",
		"bucket", bucket,
		"object", objectPath,
		"bytes", len(content))

	return &GCSToolResult{
		Success:   true,
		Operation: "read",
		Message:   fmt.Sprintf("Read %d bytes from gs://%s/%s", len(content), bucket, objectPath),
		Content:   string(content),
	}, nil
}

// handleInfoOperation gets metadata about a GCS object
func handleInfoOperation(ctx context.Context, logger *slog.Logger, client *storage.Client,
	bucket, objectPath string) (*GCSToolResult, error) {

	if objectPath == "" {
		return &GCSToolResult{
			Success:   false,
			Operation: "info",
			Error:     "Object path cannot be empty for info operation",
		}, fmt.Errorf("empty object path")
	}

	obj := client.Bucket(bucket).Object(objectPath)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		logger.Error("Failed to get object attributes", "error", err, "bucket", bucket, "object", objectPath)
		return &GCSToolResult{
			Success:   false,
			Operation: "info",
			Error:     fmt.Sprintf("Failed to get object info: %v", err),
		}, err
	}

	objectInfo := GCSObjectInfo{
		Name:         attrs.Name,
		Size:         attrs.Size,
		ContentType:  attrs.ContentType,
		Updated:      attrs.Updated,
		MD5Hash:      fmt.Sprintf("%x", attrs.MD5),
		CRC32C:       attrs.CRC32C,
		StorageClass: attrs.StorageClass,
		IsPrefix:     false,
	}

	logger.Info("Retrieved GCS object info",
		"bucket", bucket,
		"object", objectPath,
		"size", attrs.Size)

	return &GCSToolResult{
		Success:   true,
		Operation: "info",
		Message:   fmt.Sprintf("Object info for gs://%s/%s", bucket, objectPath),
		Objects:   []GCSObjectInfo{objectInfo},
	}, nil
}
