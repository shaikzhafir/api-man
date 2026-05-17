package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type WebServer struct {
	cm     *ConfigManager
	port   string
	static string
}

type APIRequest struct {
	Request     RequestData `json:"request"`
	Environment string      `json:"environment"`
	Collection  string      `json:"collection,omitempty"`
}

type RequestData struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

type APIResponse struct {
	Status  string            `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Time    string            `json:"time"`
	Curl    string            `json:"curl,omitempty"`
	Request *ExecutedRequest  `json:"request,omitempty"`
	Error   bool              `json:"error,omitempty"`
	Message string            `json:"message,omitempty"`
}

type ExecutedRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type EnvironmentInfo struct {
	Name    string `json:"name"`
	BaseURL string `json:"baseURL"`
}

func NewWebServer(port, staticDir string) (*WebServer, error) {
	cm, err := NewConfigManager()
	if err != nil {
		return nil, fmt.Errorf("creating config manager: %w", err)
	}

	return &WebServer{
		cm:     cm,
		port:   port,
		static: staticDir,
	}, nil
}

func (ws *WebServer) Start() error {
	// API routes
	http.HandleFunc("/api/environments", ws.handleEnvironments)
	http.HandleFunc("/api/collection-environments/", ws.handleCollectionEnvironments)
	http.HandleFunc("/api/requests", ws.handleRequests)
	http.HandleFunc("/api/request/", ws.handleRequestDetails)
	http.HandleFunc("/api/openapi-preview", ws.handleOpenAPIPreview)
	http.HandleFunc("/api/import-openapi", ws.handleImportOpenAPI)
	http.HandleFunc("/api/execute", ws.handleExecute)

	// Serve static files
	if ws.static != "" {
		fs := http.FileServer(http.Dir(ws.static))
		http.Handle("/", fs)
	}

	fmt.Printf("🌐 Web server starting on http://localhost:%s\n", ws.port)
	fmt.Printf("📁 Serving static files from: %s\n", ws.static)

	return http.ListenAndServe(":"+ws.port, ws.corsMiddleware(http.DefaultServeMux))
}

func (ws *WebServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (ws *WebServer) handleEnvironments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	envNames, err := ws.cm.ListEnvironments()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error listing environments: %v", err), http.StatusInternalServerError)
		return
	}

	var environments []EnvironmentInfo
	for _, name := range envNames {
		env, err := ws.cm.LoadEnvironment(name)
		if err != nil {
			continue
		}
		environments = append(environments, EnvironmentInfo{
			Name:    name,
			BaseURL: env.BaseURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(environments)
}

func (ws *WebServer) handleCollectionEnvironments(w http.ResponseWriter, r *http.Request) {
	collection := strings.TrimPrefix(r.URL.Path, "/api/collection-environments/")
	if collection == "" {
		http.Error(w, "Collection name is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		ce, err := ws.cm.LoadCollectionEnvironments(collection)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error loading collection environments: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ce)

	case http.MethodPut:
		var ce CollectionEnvironments
		if err := json.NewDecoder(r.Body).Decode(&ce); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		if err := ws.cm.SaveCollectionEnvironments(collection, &ce); err != nil {
			http.Error(w, fmt.Sprintf("Error saving collection environments: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *WebServer) handleRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requests, err := ws.cm.ListRequests()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error listing requests: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func (ws *WebServer) handleRequestDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract request path from URL
	requestPath := strings.TrimPrefix(r.URL.Path, "/api/request/")
	if requestPath == "" {
		http.Error(w, "Request path is required", http.StatusBadRequest)
		return
	}

	// Load request config
	config, err := ws.cm.LoadRequest(requestPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading request: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (ws *WebServer) handleOpenAPIPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, fmt.Sprintf("Invalid multipart upload: %v", err), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("spec")
	if err != nil {
		http.Error(w, "OpenAPI spec file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading upload: %v", err), http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "OpenAPI spec file is empty", http.StatusBadRequest)
		return
	}

	spec, err := LoadOpenAPISpecFromData(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading OpenAPI spec: %v", err), http.StatusBadRequest)
		return
	}

	preview := ws.cm.PreviewOpenAPI(spec, strings.TrimSpace(r.FormValue("collection")))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preview)
}

func (ws *WebServer) handleImportOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, fmt.Sprintf("Invalid multipart upload: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("spec")
	if err != nil {
		http.Error(w, "OpenAPI spec file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading upload: %v", err), http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "OpenAPI spec file is empty", http.StatusBadRequest)
		return
	}

	spec, err := LoadOpenAPISpecFromData(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading OpenAPI spec: %v", err), http.StatusBadRequest)
		return
	}

	opts := ImportOptions{
		OverrideName: strings.TrimSpace(r.FormValue("collection")),
		Overwrite:    r.FormValue("overwrite") == "true",
	}

	result, err := ws.cm.ImportRequestsFromOpenAPI(spec, opts)
	if err != nil {
		var existsErr *CollectionExistsError
		if errors.As(err, &existsErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"error":               "collection_exists",
				"suggestedCollection": existsErr.Name,
				"message":             fmt.Sprintf("Collection %q already exists. Choose a different name or overwrite.", existsErr.Name),
			})
			return
		}
		http.Error(w, fmt.Sprintf("Error importing requests: %v", err), http.StatusInternalServerError)
		return
	}

	specPath, specErr := ws.cm.SaveCollectionSpec(result.Collection, data, detectSpecExt(header.Filename, data))
	if specErr != nil {
		// Generated files succeeded; failure to persist the source spec is non-fatal.
		fmt.Printf("warning: failed to save source spec for %s: %v\n", result.Collection, specErr)
	} else {
		result.SpecPath = specPath
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// detectSpecExt picks "yaml" or "json" based on the uploaded filename, falling
// back to a peek at the first non-whitespace byte if the extension is missing
// or unrecognized.
func detectSpecExt(name string, data []byte) string {
	lname := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lname, ".json"):
		return "json"
	case strings.HasSuffix(lname, ".yaml"), strings.HasSuffix(lname, ".yml"):
		return "yaml"
	}
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[':
			return "json"
		}
		break
	}
	return "yaml"
}

func (ws *WebServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var apiReq APIRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Load environment (collection-specific if available, otherwise global)
	env, err := ws.cm.ResolveEnvironment(apiReq.Collection, apiReq.Environment)
	if err != nil {
		apiResponse := APIResponse{
			Error:   true,
			Message: fmt.Sprintf("Error loading environment: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiResponse)
		return
	}

	// Execute request
	startTime := time.Now()
	response, err := ws.executeHTTPRequest(apiReq.Request, env)
	duration := time.Since(startTime)

	if err != nil {
		apiResponse := APIResponse{
			Error:   true,
			Message: fmt.Sprintf("Request failed: %v", err),
			Time:    fmt.Sprintf("%dms", duration.Milliseconds()),
		}
		if response != nil {
			apiResponse.Curl = response.Curl
			apiResponse.Request = response.Request
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiResponse)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (ws *WebServer) executeHTTPRequest(reqData RequestData, env *Environment) (*APIResponse, error) {
	// Build full URL
	baseURL := env.BaseURL
	if baseURL != "" && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	fullURL := baseURL + reqData.URL

	// Create HTTP request
	var httpReq *http.Request
	var err error

	if reqData.Body != "" {
		httpReq, err = http.NewRequest(reqData.Method, fullURL, strings.NewReader(reqData.Body))
	} else {
		httpReq, err = http.NewRequest(reqData.Method, fullURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Apply environment headers
	for key, value := range env.Headers {
		if value != "" {
			httpReq.Header.Set(key, value)
		}
	}

	// Apply request headers (override environment headers)
	for key, value := range reqData.Headers {
		if value != "" {
			httpReq.Header.Set(key, value)
		}
	}

	// Apply cookies
	for name, value := range env.Cookies {
		if value != "" {
			cookie := &http.Cookie{
				Name:  name,
				Value: value,
			}
			httpReq.AddCookie(cookie)
		}
	}

	applyEnvironmentAuth(httpReq, env)

	curlCommand := buildCurlCommand(httpReq)
	executedRequest := &ExecutedRequest{
		Method: httpReq.Method,
		URL:    httpReq.URL.String(),
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	startTime := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return &APIResponse{
			Curl:    curlCommand,
			Request: executedRequest,
		}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()
	duration := time.Since(startTime)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIResponse{
			Curl:    curlCommand,
			Request: executedRequest,
		}, fmt.Errorf("reading response body: %w", err)
	}

	// Convert headers to map[string]string
	headers := make(map[string]string)
	for key, values := range resp.Header {
		headers[key] = strings.Join(values, ", ")
	}

	return &APIResponse{
		Status:  resp.Status,
		Headers: headers,
		Body:    string(body),
		Time:    fmt.Sprintf("%dms", duration.Milliseconds()),
		Curl:    curlCommand,
		Request: executedRequest,
	}, nil
}

func buildCurlCommand(req *http.Request) string {
	parts := []string{
		fmt.Sprintf("curl -X %s %s", shellQuote(req.Method), shellQuote(req.URL.String())),
	}

	headerNames := make([]string, 0, len(req.Header))
	for name := range req.Header {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)

	for _, name := range headerNames {
		values := append([]string(nil), req.Header.Values(name)...)
		sort.Strings(values)
		for _, value := range values {
			parts = append(parts, fmt.Sprintf("-H %s", shellQuote(fmt.Sprintf("%s: %s", name, value))))
		}
	}

	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(strings.NewReader(string(body)))
			if len(body) > 0 {
				parts = append(parts, fmt.Sprintf("--data-raw %s", shellQuote(string(body))))
			}
		}
	}

	return strings.Join(parts, " \\\n  ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func applyEnvironmentAuth(req *http.Request, env *Environment) {
	if env.Auth == nil {
		return
	}

	switch env.Auth["type"] {
	case "bearer":
		if token := env.Auth["token"]; token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "basic":
		username := env.Auth["username"]
		password := env.Auth["password"]
		if username != "" || password != "" {
			req.SetBasicAuth(username, password)
		}
	case "api-key":
		key := env.Auth["key"]
		header := env.Auth["header"]
		if key != "" && header != "" {
			req.Header.Set(header, key)
		}
	}
}
