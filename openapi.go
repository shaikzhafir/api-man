// openapi.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func LoadOpenAPISpec(filename string) (*openapi3.T, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return LoadOpenAPISpecFromData(data)
}

func LoadOpenAPISpecFromData(data []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec: %w", err)
	}

	// Validate but don't fail on errors — kin-openapi is OpenAPI 3.0 only,
	// so 3.1 features like `type: "null"` trigger spurious errors.
	if err := doc.Validate(context.Background()); err != nil {
		fmt.Printf("⚠️  OpenAPI validation warning (continuing anyway): %v\n", err)
	}

	return doc, nil
}

func OpenAPICollectionName(spec *openapi3.T) string {
	if spec != nil && spec.Info != nil && strings.TrimSpace(spec.Info.Title) != "" {
		return sanitizeRequestPathSegment(spec.Info.Title)
	}
	return "api"
}

