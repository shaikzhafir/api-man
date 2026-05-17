// openapi.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type APIEndpoint struct {
	Path        string
	Method      string
	Summary     string
	Description string
	Parameters  []Parameter
	RequestBody *RequestBody
	Operation   *openapi3.Operation
}

type Parameter struct {
	Name        string
	In          string // path, query, header
	Required    bool
	Description string
	Schema      *openapi3.Schema
}

type RequestBody struct {
	Required    bool
	ContentType string
	Schema      *openapi3.Schema
}

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

func GetEndpoints(spec *openapi3.T) []APIEndpoint {
	var endpoints []APIEndpoint

	for path, pathItem := range spec.Paths.Map() {
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, op := range operations {
			if op == nil {
				continue
			}

			endpoint := APIEndpoint{
				Path:        path,
				Method:      method,
				Summary:     op.Summary,
				Description: op.Description,
				Operation:   op,
			}

			// Parse parameters
			for _, param := range op.Parameters {
				if param.Value != nil {
					endpoint.Parameters = append(endpoint.Parameters, Parameter{
						Name:        param.Value.Name,
						In:          param.Value.In,
						Required:    param.Value.Required,
						Description: param.Value.Description,
						Schema:      param.Value.Schema.Value,
					})
				}
			}

			// Parse request body
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for contentType, mediaType := range op.RequestBody.Value.Content {
					endpoint.RequestBody = &RequestBody{
						Required:    op.RequestBody.Value.Required,
						ContentType: contentType,
						Schema:      mediaType.Schema.Value,
					}
					break // Just take the first content type for now
				}
			}

			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints
}
