package mcp

import "context"

// APIKeyValidator validates API keys for HTTP transport authentication.
type APIKeyValidator interface {
	Validate(ctx context.Context, apiKey string) bool
}

const (
	testKey = "please-change-me-dev-key"
)

// DEVKeyValidator is a simple validator for development and testing ONLY.
//
// WARNING: This validator uses a hardcoded key and should NEVER be used in production.
// For production deployments, implement your own APIKeyValidator with secure key storage,
// constant-time comparison, rate limiting, and proper logging.
//
// See the Security section in README.md for production implementation examples.
type DEVKeyValidator struct{}

// NewDEVKeyValidator creates a new development-only key validator.
//
// WARNING: Only use this for local development and testing. Never use in production.
func NewDEVKeyValidator() *DEVKeyValidator {
	return &DEVKeyValidator{}
}

// Validate checks if the provided API key matches the hardcoded development key.
func (v *DEVKeyValidator) Validate(ctx context.Context, apiKey string) bool {
	if apiKey == testKey {
		return true
	}
	return false
}
