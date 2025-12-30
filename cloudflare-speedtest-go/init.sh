#!/bin/bash

# Cloudflare Speed Test (Go) - Project Initialization Script

echo "=========================================="
echo "Cloudflare Speed Test (Go) - Initialization"
echo "=========================================="
echo ""

# Check Go installation
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or higher."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "✅ Go version: $GO_VERSION"
echo ""

# Download dependencies
echo "📦 Downloading dependencies..."
go mod download
if [ $? -eq 0 ]; then
    echo "✅ Dependencies downloaded successfully"
else
    echo "❌ Failed to download dependencies"
    exit 1
fi
echo ""

# Tidy dependencies
echo "🧹 Tidying dependencies..."
go mod tidy
if [ $? -eq 0 ]; then
    echo "✅ Dependencies tidied successfully"
else
    echo "❌ Failed to tidy dependencies"
    exit 1
fi
echo ""

# Build the project
echo "🔨 Building the project..."
make build
if [ $? -eq 0 ]; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi
echo ""

echo "=========================================="
echo "✅ Initialization complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Run the application: make run"
echo "2. Or run directly: ./bin/cloudflare-speedtest"
echo ""
echo "For more information, see:"
echo "- QUICKSTART.md - Quick start guide"
echo "- ARCHITECTURE.md - Project architecture"
echo "- README.md - Project overview"
