// config.go
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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

		// Get relative path from requests directory
		relPath, err := filepath.Rel(cm.requestsDir, path)
		if err != nil {
			return err
		}

		// Remove .json extension
		relPath = strings.TrimSuffix(relPath, ".json")

		// Get directory name
		dir := filepath.Dir(relPath)
		if dir == "." {
			dir = "root"
		}

		requests[dir] = append(requests[dir], relPath)

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

// Compatibility methods for TUI (using old RequestConfig structure)
type LegacyRequestConfig struct {
	Name          string            `json:"name"`
	BaseURL       string            `json:"baseURL"`
	Headers       map[string]string `json:"headers"`
	Cookies       map[string]string `json:"cookies"`
	Timeout       int               `json:"timeout"`
	UserAgent     string            `json:"userAgent"`
	Authorization string            `json:"authorization"`
	ContentType   string            `json:"contentType"`
}

func (cm *ConfigManager) GetActiveConfig() *LegacyRequestConfig {
	// Return a default legacy config for TUI compatibility
	return &LegacyRequestConfig{
		Name:          "default",
		BaseURL:       "https://api.example.com",
		Headers:       make(map[string]string),
		Cookies:       make(map[string]string),
		Timeout:       30,
		UserAgent:     "API-Man/1.0",
		ContentType:   "application/json",
		Authorization: "",
	}
}

func (cm *ConfigManager) SetActiveConfig(configName string) error {
	// For TUI compatibility, we won't actually save this config
	// In a real application, you would save it to a file or database
	return nil
}

func (cm *ConfigManager) ListConfigs() []string {
	// Return a default list for TUI compatibility
	return []string{"default"}
}

func (config *LegacyRequestConfig) GetBaseURL() string {
	baseURL := config.BaseURL
	if baseURL != "" && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return baseURL
}

func (config *LegacyRequestConfig) ApplyToRequest(req *http.Request) {
	if config.UserAgent != "" {
		req.Header.Set("User-Agent", config.UserAgent)
	}

	if config.Authorization != "" {
		req.Header.Set("Authorization", config.Authorization)
	}

	if config.ContentType != "" && (req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH") {
		req.Header.Set("Content-Type", config.ContentType)
	}

	for key, value := range config.Headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	for name, value := range config.Cookies {
		if value != "" {
			cookie := &http.Cookie{
				Name:  name,
				Value: value,
			}
			req.AddCookie(cookie)
		}
	}
}

func (config *LegacyRequestConfig) CreateHTTPClient() *http.Client {
	timeout := time.Duration(config.Timeout) * time.Second
	return &http.Client{
		Timeout: timeout,
	}
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

// GenerateRequestsFromOpenAPI generates request configs from OpenAPI spec
// Creates a folder structure based on the OpenAPI spec, then for each request,
// creates a separate folder containing a JSON file with URL, headers, and body info
func (cm *ConfigManager) GenerateRequestsFromOpenAPI(spec *openapi3.T) error {
	// First, create a main folder based on the OpenAPI spec info
	specTitle := "api"
	if spec.Info != nil && spec.Info.Title != "" {
		specTitle = strings.ToLower(strings.ReplaceAll(spec.Info.Title, " ", "-"))
	}

	specDir := filepath.Join(cm.requestsDir, specTitle)
	err := os.MkdirAll(specDir, 0755)
	if err != nil {
		return fmt.Errorf("creating spec directory %s: %w", specDir, err)
	}

	for path, pathItem := range spec.Paths.Map() {
		// Generate request for each operation
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
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
			requestName = strings.ToLower(requestName)

			// Create a separate folder for this request
			requestDir := filepath.Join(specDir, requestName)
			err = os.MkdirAll(requestDir, 0755)
			if err != nil {
				return fmt.Errorf("creating request directory %s: %w", requestDir, err)
			}

			// Create simplified request info structure
			requestInfo := struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
				Body    string            `json:"body"`
				Method  string            `json:"method"`
				Name    string            `json:"name"`
			}{
				URL:     path,
				Headers: make(map[string]string),
				Body:    "",
				Method:  method,
				Name:    requestName,
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

			// Save request info to JSON file in the request folder
			requestFile := filepath.Join(requestDir, "request.json")
			data, err := json.MarshalIndent(requestInfo, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling request info: %w", err)
			}

			err = os.WriteFile(requestFile, data, 0644)
			if err != nil {
				return fmt.Errorf("writing request file %s: %w", requestFile, err)
			}
		}
	}

	return nil
}
