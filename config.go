// config.go
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

type RequestConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Method      string                 `json:"method"`
	URL         string                 `json:"url"`
	Headers     map[string]string      `json:"headers"`
	Cookies     map[string]string      `json:"cookies"`
	Body        string                 `json:"body"`
	ActiveBody  string                 `json:"activeBody,omitempty"`
	Params      map[string]interface{} `json:"params"`
	Timeout     int                    `json:"timeout"`
}

type Environment struct {
	BaseURL   string            `json:"baseURL"`
	Headers   map[string]string `json:"headers"`
	Cookies   map[string]string `json:"cookies"`
	Auth      map[string]string `json:"auth"`
	Variables map[string]string `json:"variables"`
}

type ConfigManager struct {
	configDir       string
	requestsDir     string
	environmentsDir string
}

type OpenAPIImportResult struct {
	Collection string `json:"collection"`
	Imported   int    `json:"imported"`
}

func NewConfigManager() (*ConfigManager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current working directory: %w", err)
	}
	requestsDir := filepath.Join(cwd, "requests")
	environmentsDir := filepath.Join(cwd, "environments")

	// Create directory structure
	dirs := []string{requestsDir, environmentsDir}
	for _, dir := range dirs {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	cm := &ConfigManager{
		requestsDir:     requestsDir,
		environmentsDir: environmentsDir,
	}

	err = cm.initializeDefaultFiles()
	if err != nil {
		return nil, fmt.Errorf("initializing default files: %w", err)
	}

	return cm, nil
}

func (cm *ConfigManager) initializeDefaultFiles() error {
	// Create default environment files if they don't exist
	defaultEnvs := map[string]Environment{
		"dev": {
			BaseURL: "http://localhost:3000",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Cookies: make(map[string]string),
			Auth:    make(map[string]string),
			Variables: map[string]string{
				"host": "localhost:3000",
			},
		},
		"prod": {
			BaseURL: "https://api.example.com",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Cookies: make(map[string]string),
			Auth:    make(map[string]string),
			Variables: map[string]string{
				"host": "api.example.com",
			},
		},
	}

	for name, env := range defaultEnvs {
		envFile := filepath.Join(cm.environmentsDir, name+".json")
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			data, err := json.MarshalIndent(env, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling environment %s: %w", name, err)
			}

			err = os.WriteFile(envFile, data, 0644)
			if err != nil {
				return fmt.Errorf("writing environment file %s: %w", name, err)
			}
		}
	}

	// Create sample request if requests directory is empty
	entries, err := os.ReadDir(cm.requestsDir)
	if err != nil {
		return fmt.Errorf("reading requests directory: %w", err)
	}

	if len(entries) == 0 {
		sampleRequest := RequestConfig{
			Name:        "Get Users (this is a sample request so you wont think this is a dead project)",
			Description: "Fetch all users from the API",
			Method:      "GET",
			URL:         "/users",
			Headers:     make(map[string]string),
			Cookies:     make(map[string]string),
			Body:        "",
			Params:      make(map[string]interface{}),
			Timeout:     30,
		}

		usersDir := filepath.Join(cm.requestsDir, "users")
		err = os.MkdirAll(usersDir, 0755)
		if err != nil {
			return fmt.Errorf("creating users directory: %w", err)
		}

		return cm.SaveRequest("users/get-users", sampleRequest)
	}

	return nil
}

// SaveRequest saves a request config to a file
func (cm *ConfigManager) SaveRequest(path string, config RequestConfig) error {
	// Determine if we're using the new subdirectory structure or old flat structure
	filePath := filepath.Join(cm.requestsDir, path+".json")

	// Check if subdirectory structure exists
	subdirPath := filepath.Join(cm.requestsDir, path, "request.json")
	if _, err := os.Stat(filepath.Join(cm.requestsDir, path)); err == nil {
		// Subdirectory exists, use that structure
		filePath = subdirPath
	} else {
		// Ensure directory exists for flat structure
		dir := filepath.Dir(filePath)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// LoadRequest loads a request config from a file
func (cm *ConfigManager) LoadRequest(path string) (*RequestConfig, error) {
	// Try both formats: direct .json file and subdirectory/request.json
	filePath := filepath.Join(cm.requestsDir, path+".json")

	// If the .json file doesn't exist, try looking for request.json in a subdirectory
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		filePath = filepath.Join(cm.requestsDir, path, "request.json")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var config RequestConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &config, nil
}

// ListRequests returns all request configs organized by directory
func (cm *ConfigManager) ListRequests() (map[string][]string, error) {
	requests := make(map[string][]string)

	err := filepath.WalkDir(cm.requestsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		if d.Name() == "environments.json" {
			return nil
		}

		// Get relative path from requests directory
		relPath, err := filepath.Rel(cm.requestsDir, path)
		if err != nil {
			return err
		}

		// Remove .json extension
		relPath = strings.TrimSuffix(relPath, ".json")

		// Group by top-level folder (the yaml-spec folder)
		parts := strings.Split(relPath, string(filepath.Separator))
		var topLevel string
		if len(parts) <= 1 {
			topLevel = "root"
		} else {
			topLevel = parts[0]
		}

		requests[topLevel] = append(requests[topLevel], relPath)

		return nil
	})

	return requests, err
}

// LoadEnvironment loads an environment configuration
func (cm *ConfigManager) LoadEnvironment(name string) (*Environment, error) {
	filePath := filepath.Join(cm.environmentsDir, name+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading environment file: %w", err)
	}

	var env Environment
	err = json.Unmarshal(data, &env)
	if err != nil {
		return nil, fmt.Errorf("parsing environment file: %w", err)
	}

	return &env, nil
}

// SaveEnvironment saves an environment configuration
func (cm *ConfigManager) SaveEnvironment(name string, env Environment) error {
	filePath := filepath.Join(cm.environmentsDir, name+".json")

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling environment: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("writing environment file: %w", err)
	}

	return nil
}

// ListEnvironments returns all available environments
func (cm *ConfigManager) ListEnvironments() ([]string, error) {
	entries, err := os.ReadDir(cm.environmentsDir)
	if err != nil {
		return nil, fmt.Errorf("reading environments directory: %w", err)
	}

	var environments []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			environments = append(environments, name)
		}
	}

	return environments, nil
}

// DeleteRequest deletes a request config file
func (cm *ConfigManager) DeleteRequest(path string) error {
	filePath := filepath.Join(cm.requestsDir, path+".json")

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("deleting request file: %w", err)
	}

	return nil
}

// ExecuteRequest executes a request with an environment
func (cm *ConfigManager) ExecuteRequest(requestPath, envName string) (*http.Response, error) {
	// Load request config
	config, err := cm.LoadRequest(requestPath)
	if err != nil {
		return nil, fmt.Errorf("loading request: %w", err)
	}

	// Load environment
	env, err := cm.LoadEnvironment(envName)
	if err != nil {
		return nil, fmt.Errorf("loading environment: %w", err)
	}

	// Build full URL
	baseURL := env.BaseURL
	if baseURL != "" && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	fullURL := baseURL + config.URL

	// Replace variables in URL
	for key, value := range env.Variables {
		fullURL = strings.ReplaceAll(fullURL, "{{"+key+"}}", value)
	}

	// Determine which body to use
	bodyToUse := config.Body
	if config.ActiveBody != "" {
		// Try to load body from separate JSON file in the request directory
		requestDir := filepath.Join(cm.requestsDir, requestPath)
		if _, err := os.Stat(requestDir); err == nil {
			bodyFilePath := filepath.Join(requestDir, config.ActiveBody+".json")
			if bodyData, err := os.ReadFile(bodyFilePath); err == nil {
				bodyToUse = string(bodyData)
			}
		}
	}

	// Create request
	var req *http.Request
	if bodyToUse != "" {
		req, err = http.NewRequest(config.Method, fullURL, strings.NewReader(bodyToUse))
	} else {
		req, err = http.NewRequest(config.Method, fullURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Apply environment headers
	for key, value := range env.Headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	// Apply request-specific headers (override environment headers)
	for key, value := range config.Headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	// Apply environment cookies
	for name, value := range env.Cookies {
		if value != "" {
			cookie := &http.Cookie{
				Name:  name,
				Value: value,
			}
			req.AddCookie(cookie)
		}
	}

	// Apply request-specific cookies (override environment cookies)
	for name, value := range config.Cookies {
		if value != "" {
			cookie := &http.Cookie{
				Name:  name,
				Value: value,
			}
			req.AddCookie(cookie)
		}
	}

	// Apply authentication from environment
	if authType, exists := env.Auth["type"]; exists && authType != "" {
		switch authType {
		case "bearer":
			if token, exists := env.Auth["token"]; exists && token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "basic":
			if username, exists := env.Auth["username"]; exists {
				if password, exists := env.Auth["password"]; exists {
					req.SetBasicAuth(username, password)
				}
			}
		case "api-key":
			if key, exists := env.Auth["key"]; exists && key != "" {
				if header, exists := env.Auth["header"]; exists && header != "" {
					req.Header.Set(header, key)
				}
			}
		}
	}

	// Create HTTP client with timeout
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
	}

	return client.Do(req)
}

// SetActiveBody sets which body JSON file to use for a request
func (cm *ConfigManager) SetActiveBody(requestPath, bodyName string) error {
	config, err := cm.LoadRequest(requestPath)
	if err != nil {
		return fmt.Errorf("loading request: %w", err)
	}

	// Check if the body file exists
	requestDir := filepath.Join(cm.requestsDir, requestPath)
	bodyFilePath := filepath.Join(requestDir, bodyName+".json")

	if _, err := os.Stat(bodyFilePath); os.IsNotExist(err) {
		return fmt.Errorf("body file '%s.json' does not exist in %s", bodyName, requestPath)
	}

	config.ActiveBody = bodyName
	return cm.SaveRequest(requestPath, *config)
}

// ListBodies returns all available body JSON files for a request
func (cm *ConfigManager) ListBodies(requestPath string) ([]string, string, error) {
	config, err := cm.LoadRequest(requestPath)
	if err != nil {
		return nil, "", fmt.Errorf("loading request: %w", err)
	}

	// Look for JSON files in the request directory (excluding request.json)
	requestDir := filepath.Join(cm.requestsDir, requestPath)
	var bodyFiles []string

	if entries, err := os.ReadDir(requestDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") && entry.Name() != "request.json" {
				bodyName := strings.TrimSuffix(entry.Name(), ".json")
				bodyFiles = append(bodyFiles, bodyName)
			}
		}
	}

	return bodyFiles, config.ActiveBody, nil
}

// RemoveBody removes a body JSON file from a request directory
func (cm *ConfigManager) RemoveBody(requestPath, bodyName string) error {
	requestDir := filepath.Join(cm.requestsDir, requestPath)
	bodyFilePath := filepath.Join(requestDir, bodyName+".json")

	if _, err := os.Stat(bodyFilePath); os.IsNotExist(err) {
		return fmt.Errorf("body file '%s.json' does not exist", bodyName)
	}

	err := os.Remove(bodyFilePath)
	if err != nil {
		return fmt.Errorf("removing body file: %w", err)
	}

	// If we're removing the active body, clear it
	config, err := cm.LoadRequest(requestPath)
	if err != nil {
		return fmt.Errorf("loading request: %w", err)
	}

	if config.ActiveBody == bodyName {
		config.ActiveBody = ""
		return cm.SaveRequest(requestPath, *config)
	}

	return nil
}

// CollectionEnvironments holds per-collection environment overrides.
// Stored as requests/<collection>/environments.json
type CollectionEnvironments struct {
	Environments map[string]Environment `json:"environments"`
}

// LoadCollectionEnvironments loads the per-collection environments file.
// Returns an empty map (not an error) if the file doesn't exist yet.
func (cm *ConfigManager) LoadCollectionEnvironments(collection string) (*CollectionEnvironments, error) {
	filePath := filepath.Join(cm.requestsDir, collection, "environments.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CollectionEnvironments{Environments: make(map[string]Environment)}, nil
		}
		return nil, fmt.Errorf("reading collection environments: %w", err)
	}

	var ce CollectionEnvironments
	if err := json.Unmarshal(data, &ce); err != nil {
		return nil, fmt.Errorf("parsing collection environments: %w", err)
	}
	if ce.Environments == nil {
		ce.Environments = make(map[string]Environment)
	}
	return &ce, nil
}

// SaveCollectionEnvironments persists the per-collection environments file.
func (cm *ConfigManager) SaveCollectionEnvironments(collection string, ce *CollectionEnvironments) error {
	dir := filepath.Join(cm.requestsDir, collection)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating collection directory: %w", err)
	}

	data, err := json.MarshalIndent(ce, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling collection environments: %w", err)
	}

	filePath := filepath.Join(dir, "environments.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("writing collection environments: %w", err)
	}
	return nil
}

// ResolveEnvironment returns the collection-specific environment if it exists,
// otherwise falls back to the global environment.
func (cm *ConfigManager) ResolveEnvironment(collection, envName string) (*Environment, error) {
	if collection != "" {
		ce, err := cm.LoadCollectionEnvironments(collection)
		if err == nil {
			if env, ok := ce.Environments[envName]; ok {
				return &env, nil
			}
		}
	}
	return cm.LoadEnvironment(envName)
}

// GenerateRequestsFromOpenAPI generates request configs from OpenAPI spec.
func (cm *ConfigManager) GenerateRequestsFromOpenAPI(spec *openapi3.T) error {
	_, err := cm.ImportRequestsFromOpenAPI(spec)
	return err
}

// ImportRequestsFromOpenAPI creates a collection folder from an OpenAPI spec and
// returns the generated collection name plus imported operation count.
func (cm *ConfigManager) ImportRequestsFromOpenAPI(spec *openapi3.T) (*OpenAPIImportResult, error) {
	// First, create a main folder based on the OpenAPI spec info
	specTitle := OpenAPICollectionName(spec)

	specDir := filepath.Join(cm.requestsDir, specTitle)
	err := os.MkdirAll(specDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("creating spec directory %s: %w", specDir, err)
	}

	imported := 0
	for path, pathItem := range spec.Paths.Map() {
		// Generate request for each operation
		operations := map[string]*openapi3.Operation{
			"GET":     pathItem.Get,
			"HEAD":    pathItem.Head,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"DELETE":  pathItem.Delete,
			"OPTIONS": pathItem.Options,
			"PATCH":   pathItem.Patch,
		}

		for method, operation := range operations {
			if operation == nil {
				continue
			}

			// Generate request name
			requestName := method + "-" + strings.ReplaceAll(strings.Trim(path, "/"), "/", "-")
			if operation.OperationID != "" {
				requestName = operation.OperationID
			}
			requestName = sanitizeRequestPathSegment(requestName)

			// Create a separate folder for this request
			requestDir := filepath.Join(specDir, requestName)
			err = os.MkdirAll(requestDir, 0755)
			if err != nil {
				return nil, fmt.Errorf("creating request directory %s: %w", requestDir, err)
			}

			// Create simplified request info structure
			requestInfo := struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
				Body    string            `json:"body"`
				Method  string            `json:"method"`
				Name    string            `json:"name"`
				Params  map[string]string `json:"params,omitempty"`
			}{
				URL:     path,
				Headers: make(map[string]string),
				Body:    "",
				Method:  method,
				Name:    requestName,
				Params:  make(map[string]string),
			}

			// Add default headers based on operation
			if method == "POST" || method == "PUT" || method == "PATCH" {
				requestInfo.Headers["Content-Type"] = "application/json"
				if operation.RequestBody != nil {
					requestInfo.Body = `{
  "example": "data"
}`
				}
			}

			for _, parameterRef := range operation.Parameters {
				if parameterRef == nil || parameterRef.Value == nil {
					continue
				}
				parameter := parameterRef.Value
				if parameter.In != "query" || parameter.Name == "" {
					continue
				}
				requestInfo.Params[parameter.Name] = defaultParameterValue(parameter)
			}

			if len(requestInfo.Params) == 0 {
				requestInfo.Params = nil
			}

			// Save request info to JSON file in the request folder
			requestFile := filepath.Join(requestDir, "request.json")
			data, err := json.MarshalIndent(requestInfo, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshaling request info: %w", err)
			}

			err = os.WriteFile(requestFile, data, 0644)
			if err != nil {
				return nil, fmt.Errorf("writing request file %s: %w", requestFile, err)
			}
			imported++
		}
	}

	return &OpenAPIImportResult{Collection: specTitle, Imported: imported}, nil
}

func defaultParameterValue(parameter *openapi3.Parameter) string {
	if parameter.Example != nil {
		return fmt.Sprint(parameter.Example)
	}
	if parameter.Schema != nil && parameter.Schema.Value != nil && parameter.Schema.Value.Default != nil {
		return fmt.Sprint(parameter.Schema.Value.Default)
	}
	return ""
}

var requestPathSegmentPattern = regexp.MustCompile(`[^a-z0-9._{}-]+`)

func sanitizeRequestPathSegment(value string) string {
	sanitized := strings.ToLower(strings.TrimSpace(value))
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = requestPathSegmentPattern.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "api"
	}
	return sanitized
}
