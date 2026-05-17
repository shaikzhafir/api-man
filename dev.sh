#!/bin/bash
set -euo pipefail

# Development script for running both backend and frontend
echo "🚀 Starting API-Man development environment..."

BACKEND_PID=""
FRONTEND_PID=""

# Function to clean up processes when script is interrupted
cleanup() {
    echo ""
    echo "🛑 Stopping development servers..."
    if [ -n "${BACKEND_PID:-}" ]; then
        pkill -P "$BACKEND_PID" 2>/dev/null || true
        kill "$BACKEND_PID" 2>/dev/null || true
    fi
    if [ -n "${FRONTEND_PID:-}" ]; then
        pkill -P "$FRONTEND_PID" 2>/dev/null || true
        kill "$FRONTEND_PID" 2>/dev/null || true
    fi
    exit 0
}

require_entr() {
    if ! command -v entr >/dev/null 2>&1; then
        echo "Error: entr is required for backend reloads. Install it with: brew install entr" >&2
        exit 1
    fi
}

backend_dev() {
    find . \
        \( -path ./frontend -o -path ./.git \) -prune -o \
        \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -print |
        entr -nr sh -c 'go run . web 3001 ./frontend/dist'
}

# Set up signal handling
trap cleanup SIGINT SIGTERM

require_entr

# Start the backend server on port 3001 with reloads on Go changes
echo "📡 Starting backend server on port 3001 with entr reloads..."
backend_dev &
BACKEND_PID=$!

# Wait a moment for backend to start
sleep 2

# Start the frontend development server
echo "⚛️  Starting frontend development server..."
cd frontend && npm run dev &
FRONTEND_PID=$!
cd ..

echo ""
echo "✅ Development environment ready!"
echo "🌐 Frontend: http://localhost:5173"
echo "📡 Backend:  http://localhost:3001"
echo "📋 API:      http://localhost:3001/api/"
echo ""
echo "Press Ctrl+C to stop all servers"

# Wait for servers to run
wait
