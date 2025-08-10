// http.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func SendHTTPRequest(config *LegacyRequestConfig, endpoint APIEndpoint, params map[string]string, body string) (string, error) {
	// Build URL
	baseURL := config.GetBaseURL()
	url := baseURL + endpoint.Path

	// Replace path parameters
	for _, param := range endpoint.Parameters {
		if param.In == "path" {
			if value, ok := params[param.Name]; ok {
				url = strings.ReplaceAll(url, "{"+param.Name+"}", value)
			}
		}
	}

	// Add query parameters
	queryParams := []string{}
	for _, param := range endpoint.Parameters {
		if param.In == "query" {
			if value, ok := params[param.Name]; ok && value != "" {
				queryParams = append(queryParams, fmt.Sprintf("%s=%s", param.Name, value))
			}
		}
	}
	if len(queryParams) > 0 {
		url += "?" + strings.Join(queryParams, "&")
	}

	// Create request
	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(endpoint.Method, url, bytes.NewBufferString(body))
		if err != nil {
			return "", err
		}
	} else {
		req, err = http.NewRequest(endpoint.Method, url, nil)
		if err != nil {
			return "", err
		}
	}

	// Apply configuration to request (headers, cookies, etc.)
	config.ApplyToRequest(req)

	// Add header parameters
	for _, param := range endpoint.Parameters {
		if param.In == "header" {
			if value, ok := params[param.Name]; ok && value != "" {
				req.Header.Set(param.Name, value)
			}
		}
	}

	// Send request using configured client
	client := config.CreateHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Format response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Status: %s\n", resp.Status))
	result.WriteString("Headers:\n")
	for key, values := range resp.Header {
		result.WriteString(fmt.Sprintf("  %s: %s\n", key, strings.Join(values, ", ")))
	}
	result.WriteString("\nBody:\n")

	// Try to pretty print JSON
	var jsonData interface{}
	if err := json.Unmarshal(respBody, &jsonData); err == nil {
		prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
		if err == nil {
			result.WriteString(string(prettyJSON))
		} else {
			result.WriteString(string(respBody))
		}
	} else {
		result.WriteString(string(respBody))
	}

	return result.String(), nil
}
