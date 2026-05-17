# API-Man - Postman-like API Client

A filesystem-based API request management tool with a web-based Postman-like UI built with Go and React.

## Features

### Command Line Interface (CLI)
- **Initialize workspace**: Set up request and environment directories
- **Generate requests**: Auto-generate from OpenAPI specifications
- **Execute requests**: Run API calls from command line
- **Manage environments**: Switch between dev, staging, prod configurations
- **Body templates**: Manage multiple JSON body templates per request

### Web Interface
- **Postman-like UI**: Modern web interface for API testing
- **Visual request builder**: Method selection, URL input, headers, and body editing
- **Environment switching**: Easy environment selection from dropdown
- **Response viewer**: Formatted JSON responses with headers and status codes
- **Request collections**: Organized folder structure for your APIs
- **Real-time execution**: Send requests and see responses instantly

## Quick Start

### Prerequisites
- Go 1.19+ installed
- Node.js 18+ installed (for web interface)
- entr installed for backend reloads (`brew install entr`)

### Setup
1. **Build the application:**
   ```bash
   go build -o api-man
   ```

2. **Initialize workspace:**
   ```bash
   ./api-man init
   ```

3. **Generate requests from OpenAPI spec (optional):**
   ```bash
   ./api-man generate your-spec.yaml
   ```

### Using the Web Interface

#### Production Mode
1. **Build the frontend:**
   ```bash
   cd frontend && npm install && npm run build && cd ..
   ```

2. **Start the web server:**
   ```bash
   ./api-man web
   ```

3. **Open your browser:**
   - Navigate to http://localhost:3000
   - Use the Postman-like interface to test your APIs

#### Development Mode
For backend and frontend development with hot reload:

1. **Start development environment:**
   ```bash
   ./dev.sh
   ```

2. **Access the application:**
   - Frontend (hot reload): http://localhost:5173
   - Backend API (reloads on Go changes): http://localhost:3001

### Using the CLI

#### Basic Commands
```bash
# List all requests
./api-man list

# List environments
./api-man envs

# Execute a request
./api-man run booktrackr-api/get-me dev

# Start web server
./api-man web [port] [static-dir]
```

#### Body Template Management
```bash
# List body templates for a request
./api-man body list booktrackr-api/post-login

# Set active body template
./api-man body set booktrackr-api/post-login admin-user

# Remove body template
./api-man body remove booktrackr-api/post-login admin-user
```

## Project Structure

```
api-man/
├── main.go              # Main CLI application
├── config.go           # Configuration management
├── webserver.go        # Web server for UI
├── openapi.go         # OpenAPI spec parsing
├── requests/          # Request definitions
│   └── [collection]/
│       └── [request]/
│           ├── request.json
│           ├── user1.json
│           └── admin.json
├── environments/      # Environment configs
│   ├── dev.json
│   ├── staging.json
│   └── prod.json
└── frontend/         # React web interface
    ├── src/
    │   ├── components/
    │   │   ├── RequestBuilder.jsx
    │   │   ├── RequestList.jsx
    │   │   ├── ResponseDisplay.jsx
    │   │   └── EnvironmentSelector.jsx
    │   └── App.jsx
    └── dist/         # Built static files
```

## Configuration

### Environment Files
Located in `environments/`, these JSON files define:
- Base URLs
- Default headers
- Authentication settings
- Environment variables

Example `dev.json`:
```json
{
  "baseURL": "http://localhost:8080",
  "headers": {
    "Content-Type": "application/json"
  },
  "auth": {
    "type": "bearer",
    "token": "your-dev-token"
  },
  "variables": {
    "host": "localhost:8080"
  }
}
```

### Request Files
Located in `requests/[collection]/[request-name]/`, these define individual API calls:

`request.json`:
```json
{
  "name": "Get User Profile",
  "method": "GET",
  "url": "/me",
  "headers": {},
  "timeout": 30
}
```

Multiple body templates can be added as separate JSON files in the same directory.

## Web Interface Features

### Request Builder
- **Method Selection**: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
- **URL Input**: Full URL path with parameter support
- **Headers Management**: Add/remove custom headers with key-value pairs
- **Body Editor**: JSON body editing with syntax validation
- **Tabbed Interface**: Switch between headers and body configuration

### Request Collections
- **Folder Organization**: Requests organized in expandable folders
- **Visual Method Badges**: Color-coded HTTP method indicators
- **Search/Filter**: Easy browsing of request collections
- **Request Count**: Shows number of requests per collection

### Response Viewer
- **Status Display**: HTTP status codes with color coding
- **Response Time**: Execution time in milliseconds
- **Tabbed Content**: Switch between response body and headers
- **JSON Formatting**: Pretty-printed JSON responses
- **Header Details**: Complete response header information

### Environment Management
- **Environment Dropdown**: Easy switching between configurations
- **Visual Indicators**: Current environment and base URL display
- **Dynamic Loading**: Environments loaded from backend configuration

## API Endpoints

The web server exposes these REST endpoints:

- `GET /api/environments` - List all environments
- `GET /api/requests` - List all request collections
- `GET /api/request/[path]` - Get specific request details
- `POST /api/execute` - Execute a request with environment

## Development

### Frontend Development
```bash
cd frontend
npm install
npm run dev    # Development server with hot reload
npm run build  # Production build
```

### Backend Development
```bash
make backend-dev  # Start backend server with entr reloads
```

### Full Development Environment
```bash
./dev.sh  # Starts reloading backend (port 3001) and frontend (port 5173)
```

## Examples

### Testing a Login Endpoint
1. Select "booktrackr-api" → "post login" from the request list
2. Choose "dev" environment
3. Modify the request body with test credentials
4. Click "Send" to execute
5. View formatted response with status and headers

### Managing Multiple Environments
1. Create `staging.json` and `prod.json` in `environments/`
2. Switch between environments using the dropdown
3. Each environment can have different base URLs and auth tokens

### Custom Headers
1. Go to Headers tab in request builder
2. Add API keys, content types, or custom headers
3. Headers are applied per request and can override environment defaults

## License

This tool is designed for API development and testing workflows, providing both command-line efficiency and web interface convenience.
