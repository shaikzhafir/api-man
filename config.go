// config.go
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	Bodies     int    `json:"bodies,omitempty"`
	Pruned     int    `json:"pruned,omitempty"`
	SpecPath   string `json:"specPath,omitempty"`
}

// OpenAPIPreview describes what would happen if a spec were imported,
// without writing anything to disk.
type OpenAPIPreview struct {
	SuggestedCollection string `json:"suggestedCollection"`
	Exists              bool   `json:"exists"`
	OwnedBySpec         bool   `json:"ownedBySpec"`
	Operations          int    `json:"operations"`
	Requests            int    `json:"requests,omitempty"`
	Bodies              int    `json:"bodies,omitempty"`
	Type                string `json:"type,omitempty"`
}

// ImportOptions controls OpenAPI import behavior.
type ImportOptions struct {
	// OverrideName replaces the auto-derived collection folder name when non-empty.
	OverrideName string
	// Overwrite permits importing into an existing collection folder.
	// When false, attempts to import into an existing folder return CollectionExistsError.
	Overwrite bool
}

// CollectionExistsError is returned when the target collection folder already
// exists and the caller did not request overwrite.
type CollectionExistsError struct {
	Name string
}

func (e *CollectionExistsError) Error() string {
	return fmt.Sprintf("collection %q already exists", e.Name)
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
		configDir:       cwd,
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

		switch d.Name() {
		case "environments.json", "openapi.json", "openapi.yaml", "openapi.yml":
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

var environmentNamePattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

// ValidateEnvironmentName enforces the on-disk naming rule for environment files.
// Names map directly to environments/<name>.json so they must be filesystem-safe.
func ValidateEnvironmentName(name string) error {
	if name == "" {
		return fmt.Errorf("name required")
	}
	if len(name) > 64 {
		return fmt.Errorf("name must be 64 characters or fewer")
	}
	if !environmentNamePattern.MatchString(name) {
		return fmt.Errorf("name must be lowercase letters, numbers, dot, dash, or underscore")
	}
	return nil
}

// EnvironmentExistsError signals an attempt to create an env that already exists on disk.
type EnvironmentExistsError struct {
	Name string
}

func (e *EnvironmentExistsError) Error() string {
	return fmt.Sprintf("environment %q already exists", e.Name)
}

// CreateEnvironment writes a new environment file. When source is non-empty,
// values are cloned from that existing environment; otherwise a minimal default
// with Content-Type: application/json is seeded.
func (cm *ConfigManager) CreateEnvironment(name, source string) (*Environment, error) {
	if err := ValidateEnvironmentName(name); err != nil {
		return nil, err
	}

	filePath := filepath.Join(cm.environmentsDir, name+".json")
	if _, err := os.Stat(filePath); err == nil {
		return nil, &EnvironmentExistsError{Name: name}
	}

	var env Environment
	if source != "" {
		src, err := cm.LoadEnvironment(source)
		if err != nil {
			return nil, fmt.Errorf("loading source environment %q: %w", source, err)
		}
		env = cloneEnvironment(src)
	} else {
		env = Environment{
			BaseURL:   "",
			Headers:   map[string]string{"Content-Type": "application/json"},
			Cookies:   map[string]string{},
			Auth:      map[string]string{},
			Variables: map[string]string{},
		}
	}

	if err := cm.SaveEnvironment(name, env); err != nil {
		return nil, err
	}
	return &env, nil
}

func cloneEnvironment(src *Environment) Environment {
	dst := Environment{
		BaseURL:   src.BaseURL,
		Headers:   make(map[string]string, len(src.Headers)),
		Cookies:   make(map[string]string, len(src.Cookies)),
		Auth:      make(map[string]string, len(src.Auth)),
		Variables: make(map[string]string, len(src.Variables)),
	}
	maps.Copy(dst.Headers, src.Headers)
	maps.Copy(dst.Cookies, src.Cookies)
	maps.Copy(dst.Auth, src.Auth)
	maps.Copy(dst.Variables, src.Variables)
	return dst
}

// Body templates are named *.json files that live next to a request's
// request.json file (path: requests/<col>/<req>/<name>.json). The reserved name
// "default" refers to the inline RequestConfig.Body field, not a file. The
// regex below matches the env naming rule for consistency.
var bodyNamePattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

const defaultBodyName = "default"

// ValidateBodyName enforces the on-disk naming rule for body templates.
// "default" is reserved for the inline body field on RequestConfig.
func ValidateBodyName(name string) error {
	if name == "" {
		return fmt.Errorf("name required")
	}
	if name == defaultBodyName {
		return fmt.Errorf("name %q is reserved", defaultBodyName)
	}
	if len(name) > 64 {
		return fmt.Errorf("name must be 64 characters or fewer")
	}
	if !bodyNamePattern.MatchString(name) {
		return fmt.Errorf("name must be lowercase letters, numbers, dot, dash, or underscore")
	}
	return nil
}

// BodyExistsError signals an attempt to create a body template that already exists.
type BodyExistsError struct {
	Name string
}

func (e *BodyExistsError) Error() string {
	return fmt.Sprintf("body %q already exists", e.Name)
}

// LoadBodyContent reads the raw content of a named body template. The reserved
// name "default" returns the inline RequestConfig.Body value.
func (cm *ConfigManager) LoadBodyContent(requestPath, name string) (string, error) {
	if name == defaultBodyName || name == "" {
		config, err := cm.LoadRequest(requestPath)
		if err != nil {
			return "", err
		}
		return config.Body, nil
	}
	if err := ValidateBodyName(name); err != nil {
		return "", err
	}
	bodyFilePath := filepath.Join(cm.requestsDir, requestPath, name+".json")
	data, err := os.ReadFile(bodyFilePath)
	if err != nil {
		return "", fmt.Errorf("reading body file: %w", err)
	}
	return string(data), nil
}

// SaveBodyContent writes the content for a named body template. Saving the
// reserved name "default" updates the inline RequestConfig.Body field.
func (cm *ConfigManager) SaveBodyContent(requestPath, name, content string) error {
	if name == defaultBodyName || name == "" {
		config, err := cm.LoadRequest(requestPath)
		if err != nil {
			return err
		}
		config.Body = content
		return cm.SaveRequest(requestPath, *config)
	}
	if err := ValidateBodyName(name); err != nil {
		return err
	}
	requestDir := filepath.Join(cm.requestsDir, requestPath)
	if err := os.MkdirAll(requestDir, 0755); err != nil {
		return fmt.Errorf("creating request directory: %w", err)
	}
	bodyFilePath := filepath.Join(requestDir, name+".json")
	if err := os.WriteFile(bodyFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing body file: %w", err)
	}
	return nil
}

// CreateBody creates a new body template file. When source is non-empty its
// content is cloned; "default" or "" clones the inline body. Returns
// BodyExistsError if the named template already exists.
func (cm *ConfigManager) CreateBody(requestPath, name, source string) (string, error) {
	if err := ValidateBodyName(name); err != nil {
		return "", err
	}
	requestDir := filepath.Join(cm.requestsDir, requestPath)
	bodyFilePath := filepath.Join(requestDir, name+".json")
	if _, err := os.Stat(bodyFilePath); err == nil {
		return "", &BodyExistsError{Name: name}
	}

	var content string
	if source != "" {
		seed, err := cm.LoadBodyContent(requestPath, source)
		if err != nil {
			return "", fmt.Errorf("loading source body %q: %w", source, err)
		}
		content = seed
	}

	if err := cm.SaveBodyContent(requestPath, name, content); err != nil {
		return "", err
	}
	return content, nil
}

// SetActiveBodyByName sets which body to use for a request. The reserved name
// "default" or an empty string clears ActiveBody so the inline body is used.
func (cm *ConfigManager) SetActiveBodyByName(requestPath, name string) error {
	config, err := cm.LoadRequest(requestPath)
	if err != nil {
		return fmt.Errorf("loading request: %w", err)
	}
	if name == "" || name == defaultBodyName {
		config.ActiveBody = ""
		return cm.SaveRequest(requestPath, *config)
	}
	if err := ValidateBodyName(name); err != nil {
		return err
	}
	bodyFilePath := filepath.Join(cm.requestsDir, requestPath, name+".json")
	if _, err := os.Stat(bodyFilePath); os.IsNotExist(err) {
		return fmt.Errorf("body file %q does not exist", name)
	}
	config.ActiveBody = name
	return cm.SaveRequest(requestPath, *config)
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

// SaveCollectionSpec writes the source OpenAPI spec into the collection
// folder so the generated request files have a visible, on-disk origin.
// The ext argument should be "yaml" or "json"; the file lands at
// requests/<collection>/openapi.<ext>. Any previous spec file in either
// format is removed so re-imports don't leave stale variants behind.
func (cm *ConfigManager) SaveCollectionSpec(collection string, data []byte, ext string) (string, error) {
	if collection == "" {
		return "", fmt.Errorf("collection name is required")
	}
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext != "yaml" && ext != "yml" && ext != "json" {
		ext = "yaml"
	}

	dir := filepath.Join(cm.requestsDir, collection)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating collection directory: %w", err)
	}

	// Remove any prior spec variants so the collection holds exactly one canonical source.
	for _, variant := range []string{"openapi.yaml", "openapi.yml", "openapi.json"} {
		_ = os.Remove(filepath.Join(dir, variant))
	}

	filename := "openapi." + ext
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("writing spec file: %w", err)
	}

	rel, err := filepath.Rel(cm.configDir, filePath)
	if err != nil {
		return filePath, nil
	}
	return rel, nil
}

const collectionExportFormat = "api-man.collection.v1"

// CollectionExport is the portable API-Man collection bundle format. It
// captures request.json files plus sibling named body template files.
type CollectionExport struct {
	Format       string                  `json:"format"`
	Collection   string                  `json:"collection"`
	ExportedAt   string                  `json:"exportedAt"`
	Requests     []CollectionExportItem  `json:"requests"`
	Environments *CollectionEnvironments `json:"environments,omitempty"`
	SourceSpec   *CollectionExportFile   `json:"sourceSpec,omitempty"`
}

type CollectionExportItem struct {
	Path   string            `json:"path"`
	Config RequestConfig     `json:"config"`
	Bodies map[string]string `json:"bodies,omitempty"`
}

type CollectionExportFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// IsCollectionExport reports whether raw JSON looks like an API-Man collection
// bundle. It intentionally checks only the format marker so callers can fall
// back to OpenAPI parsing for ordinary JSON specs.
func IsCollectionExport(data []byte) bool {
	var probe struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Format == collectionExportFormat
}

func ParseCollectionExport(data []byte) (*CollectionExport, error) {
	var bundle CollectionExport
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parsing collection bundle: %w", err)
	}
	if bundle.Format != collectionExportFormat {
		return nil, fmt.Errorf("unsupported collection bundle format %q", bundle.Format)
	}
	if strings.TrimSpace(bundle.Collection) == "" {
		return nil, fmt.Errorf("collection bundle is missing collection")
	}
	return &bundle, nil
}

// ExportCollection builds a portable collection bundle from requests/<collection>.
func (cm *ConfigManager) ExportCollection(collection string) (*CollectionExport, error) {
	if err := validateCollectionName(collection); err != nil {
		return nil, err
	}

	collectionDir := filepath.Join(cm.requestsDir, collection)
	info, err := os.Stat(collectionDir)
	if err != nil {
		return nil, fmt.Errorf("reading collection: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("collection %q is not a directory", collection)
	}

	bundle := &CollectionExport{
		Format:     collectionExportFormat,
		Collection: collection,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Requests:   []CollectionExportItem{},
	}

	if ce, err := cm.LoadCollectionEnvironments(collection); err == nil && len(ce.Environments) > 0 {
		bundle.Environments = ce
	}

	for _, specName := range []string{"openapi.yaml", "openapi.yml", "openapi.json"} {
		specPath := filepath.Join(collectionDir, specName)
		data, err := os.ReadFile(specPath)
		if err == nil {
			bundle.SourceSpec = &CollectionExportFile{Name: specName, Content: string(data)}
			break
		}
	}

	err = filepath.WalkDir(collectionDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "request.json" {
			return nil
		}

		requestDir := filepath.Dir(path)
		rel, err := filepath.Rel(collectionDir, requestDir)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading request %s: %w", path, err)
		}
		var config RequestConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing request %s: %w", path, err)
		}

		item := CollectionExportItem{
			Path:   filepath.ToSlash(rel),
			Config: config,
			Bodies: map[string]string{},
		}

		entries, err := os.ReadDir(requestDir)
		if err != nil {
			return fmt.Errorf("reading request directory %s: %w", requestDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "request.json" {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".json")
			content, err := os.ReadFile(filepath.Join(requestDir, entry.Name()))
			if err != nil {
				return fmt.Errorf("reading body %s/%s: %w", item.Path, entry.Name(), err)
			}
			item.Bodies[name] = string(content)
		}
		if len(item.Bodies) == 0 {
			item.Bodies = nil
		}
		bundle.Requests = append(bundle.Requests, item)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(bundle.Requests, func(i, j int) bool {
		return bundle.Requests[i].Path < bundle.Requests[j].Path
	})

	return bundle, nil
}

func (cm *ConfigManager) PreviewCollectionExport(bundle *CollectionExport, overrideName string) *OpenAPIPreview {
	name := sanitizeRequestPathSegment(bundle.Collection)
	if strings.TrimSpace(overrideName) != "" {
		name = sanitizeRequestPathSegment(overrideName)
	}
	exists, ownedBySpec := inspectCollectionDir(filepath.Join(cm.requestsDir, name))

	bodyCount := 0
	for _, request := range bundle.Requests {
		bodyCount += len(request.Bodies)
	}

	return &OpenAPIPreview{
		SuggestedCollection: name,
		Exists:              exists,
		OwnedBySpec:         ownedBySpec,
		Operations:          len(bundle.Requests),
		Requests:            len(bundle.Requests),
		Bodies:              bodyCount,
		Type:                "collection",
	}
}

// ImportCollectionExport restores a portable API-Man collection bundle.
func (cm *ConfigManager) ImportCollectionExport(bundle *CollectionExport, opts ImportOptions) (*OpenAPIImportResult, error) {
	if bundle == nil {
		return nil, fmt.Errorf("collection bundle is required")
	}
	if bundle.Format != collectionExportFormat {
		return nil, fmt.Errorf("unsupported collection bundle format %q", bundle.Format)
	}

	collection := sanitizeRequestPathSegment(bundle.Collection)
	if strings.TrimSpace(opts.OverrideName) != "" {
		collection = sanitizeRequestPathSegment(opts.OverrideName)
	}
	if err := validateCollectionName(collection); err != nil {
		return nil, err
	}

	collectionDir := filepath.Join(cm.requestsDir, collection)
	exists, _ := inspectCollectionDir(collectionDir)
	if exists {
		if !opts.Overwrite {
			return nil, &CollectionExistsError{Name: collection}
		}
		if err := os.RemoveAll(collectionDir); err != nil {
			return nil, fmt.Errorf("removing existing collection: %w", err)
		}
	}
	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		return nil, fmt.Errorf("creating collection directory: %w", err)
	}

	if bundle.Environments != nil {
		if err := cm.SaveCollectionEnvironments(collection, bundle.Environments); err != nil {
			return nil, err
		}
	}

	specPath := ""
	if bundle.SourceSpec != nil && bundle.SourceSpec.Name != "" {
		name := filepath.Base(bundle.SourceSpec.Name)
		switch name {
		case "openapi.yaml", "openapi.yml", "openapi.json":
			path := filepath.Join(collectionDir, name)
			if err := os.WriteFile(path, []byte(bundle.SourceSpec.Content), 0644); err != nil {
				return nil, fmt.Errorf("writing source spec: %w", err)
			}
			if rel, err := filepath.Rel(cm.configDir, path); err == nil {
				specPath = rel
			} else {
				specPath = path
			}
		}
	}

	imported := 0
	bodyCount := 0
	for _, request := range bundle.Requests {
		relPath, err := cleanBundleRequestPath(request.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid request path %q: %w", request.Path, err)
		}

		requestDir := filepath.Join(collectionDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(requestDir, 0755); err != nil {
			return nil, fmt.Errorf("creating request directory: %w", err)
		}

		data, err := json.MarshalIndent(request.Config, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling request %s: %w", request.Path, err)
		}
		if err := os.WriteFile(filepath.Join(requestDir, "request.json"), data, 0644); err != nil {
			return nil, fmt.Errorf("writing request %s: %w", request.Path, err)
		}

		for name, content := range request.Bodies {
			if err := ValidateBodyName(name); err != nil {
				return nil, fmt.Errorf("invalid body name %q for %s: %w", name, request.Path, err)
			}
			if err := os.WriteFile(filepath.Join(requestDir, name+".json"), []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("writing body %s/%s.json: %w", request.Path, name, err)
			}
			bodyCount++
		}
		imported++
	}

	return &OpenAPIImportResult{
		Collection: collection,
		Imported:   imported,
		Bodies:     bodyCount,
		SpecPath:   specPath,
	}, nil
}

func validateCollectionName(collection string) error {
	if strings.TrimSpace(collection) == "" {
		return fmt.Errorf("collection name is required")
	}
	if filepath.Base(collection) != collection || collection == "." || collection == ".." {
		return fmt.Errorf("collection name must be a single folder name")
	}
	return nil
}

func cleanBundleRequestPath(path string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path must be relative")
	}
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("path cannot contain '..'")
		}
	}
	return filepath.ToSlash(cleaned), nil
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
// CLI semantics: regenerate-on-spec-change, so existing folders are overwritten.
func (cm *ConfigManager) GenerateRequestsFromOpenAPI(spec *openapi3.T) error {
	_, err := cm.ImportRequestsFromOpenAPI(spec, ImportOptions{Overwrite: true})
	return err
}

// ImportRequestsFromOpenAPI creates or updates a collection folder from an
// OpenAPI spec. The folder name defaults to the sanitized spec title; supply
// opts.OverrideName to choose a different folder.
//
// Conflict policy: a folder containing an openapi.{yaml,yml,json} file is
// considered "spec-owned" and is merged in place silently. A folder without
// such a file is treated as foreign; the import returns CollectionExistsError
// unless opts.Overwrite is true.
//
// Merge policy: every operation in the spec rewrites its <op>/request.json.
// Sibling files inside an operation folder (body templates) are preserved.
// Operation folders not present in the new spec are pruned entirely.
func (cm *ConfigManager) ImportRequestsFromOpenAPI(spec *openapi3.T, opts ImportOptions) (*OpenAPIImportResult, error) {
	specTitle := OpenAPICollectionName(spec)
	if strings.TrimSpace(opts.OverrideName) != "" {
		specTitle = sanitizeRequestPathSegment(opts.OverrideName)
	}

	specDir := filepath.Join(cm.requestsDir, specTitle)
	exists, ownedBySpec := inspectCollectionDir(specDir)
	if exists && !ownedBySpec && !opts.Overwrite {
		return nil, &CollectionExistsError{Name: specTitle}
	}

	err := os.MkdirAll(specDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("creating spec directory %s: %w", specDir, err)
	}

	imported := 0
	writtenOps := make(map[string]struct{})
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
			writtenOps[requestName] = struct{}{}
			imported++
		}
	}

	pruned, perr := pruneStaleOperations(specDir, writtenOps)
	if perr != nil {
		return nil, fmt.Errorf("pruning stale operations: %w", perr)
	}

	return &OpenAPIImportResult{Collection: specTitle, Imported: imported, Pruned: pruned}, nil
}

// inspectCollectionDir reports whether the directory exists and, if so,
// whether it carries a sibling openapi.{yaml,yml,json} file marking it as
// spec-owned.
func inspectCollectionDir(dir string) (exists bool, ownedBySpec bool) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false, false
	}
	for _, variant := range []string{"openapi.yaml", "openapi.yml", "openapi.json"} {
		if _, err := os.Stat(filepath.Join(dir, variant)); err == nil {
			return true, true
		}
	}
	return true, false
}

// pruneStaleOperations removes operation subdirectories under collectionDir
// whose name is not in writtenOps. A subdirectory is treated as an operation
// folder only when it contains a request.json file.
func pruneStaleOperations(collectionDir string, writtenOps map[string]struct{}) (int, error) {
	entries, err := os.ReadDir(collectionDir)
	if err != nil {
		return 0, fmt.Errorf("reading collection directory: %w", err)
	}

	pruned := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, kept := writtenOps[entry.Name()]; kept {
			continue
		}
		opDir := filepath.Join(collectionDir, entry.Name())
		if _, err := os.Stat(filepath.Join(opDir, "request.json")); err != nil {
			// Not an operation folder (no request.json). Leave it alone.
			continue
		}
		if err := os.RemoveAll(opDir); err != nil {
			return pruned, fmt.Errorf("removing %s: %w", opDir, err)
		}
		pruned++
	}
	return pruned, nil
}

// PreviewOpenAPI parses an OpenAPI document and reports what an import would
// do, without touching disk. overrideName, when non-empty, replaces the
// default folder name derived from spec.info.title.
func (cm *ConfigManager) PreviewOpenAPI(spec *openapi3.T, overrideName string) *OpenAPIPreview {
	name := OpenAPICollectionName(spec)
	if strings.TrimSpace(overrideName) != "" {
		name = sanitizeRequestPathSegment(overrideName)
	}

	exists, ownedBySpec := inspectCollectionDir(filepath.Join(cm.requestsDir, name))

	operations := 0
	if spec.Paths != nil {
		for _, pathItem := range spec.Paths.Map() {
			if pathItem == nil {
				continue
			}
			for _, op := range []*openapi3.Operation{
				pathItem.Get, pathItem.Head, pathItem.Post, pathItem.Put,
				pathItem.Delete, pathItem.Options, pathItem.Patch,
			} {
				if op != nil {
					operations++
				}
			}
		}
	}

	return &OpenAPIPreview{
		SuggestedCollection: name,
		Exists:              exists,
		OwnedBySpec:         ownedBySpec,
		Operations:          operations,
	}
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
