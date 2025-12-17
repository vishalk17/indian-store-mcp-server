#!/bin/bash

# Check if .env exists
if [ ! -f .env ]; then
    echo "⚠️  .env file not found!"
    echo "📝 Creating .env from .env.example..."
    cp .env.example .env
    echo "✅ .env created. Please edit it with your Ory credentials:"
    echo "   - ORY_URL"
    echo "   - ORY_CLIENT_ID"
    echo "   - ORY_CLIENT_SECRET"
    exit 1
fi

# Load environment variables
export $(grep -v '^#' .env | xargs)

echo "🚀 Starting Indian Store MCP Server with Ory OAuth..."
echo "📍 Server: http://${HOST}:${PORT}"
echo "🔐 OAuth: ${ORY_URL}"
echo ""

# Run the server
go run main.go
