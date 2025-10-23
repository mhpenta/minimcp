package mcp

import "context"

type APIKeyValidator interface {
	Validate(ctx context.Context, apiKey string) bool
}

const (
	testKey = "please-change-me-dev-key"
)

type DEVKeyValidator struct{}

func NewDEVKeyValidator() *DEVKeyValidator {
	return &DEVKeyValidator{}
}

func (v *DEVKeyValidator) Validate(ctx context.Context, apiKey string) bool {
	if apiKey == testKey {
		return true
	}
	return false
}
