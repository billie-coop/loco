#!/bin/bash

# Test script for Loco tools
echo "🧪 Loco Tool Testing"
echo "==================="
echo ""

# Check if LM Studio is running
echo "Checking LM Studio..."
if curl -s http://localhost:1234/v1/models > /dev/null; then
    echo "✅ LM Studio is running"
    echo ""
    echo "Available models:"
    curl -s http://localhost:1234/v1/models | grep -o '"id":"[^"]*"' | cut -d'"' -f4 | sed 's/^/  - /'
else
    echo "❌ LM Studio is not running!"
    echo "Please start LM Studio and load a model"
    exit 1
fi

echo ""
echo "Building Loco..."
if go build -o loco; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

echo ""
echo "Ready to test! Here are some test prompts to try:"
echo ""
echo "1. Basic file reading:"
echo '   "Show me the main.go file"'
echo '   "Read the README"'
echo '   "What\'s in the parser.go file?"'
echo ""
echo "2. Directory listing:"
echo '   "List the files in internal/"'
echo '   "Show me what\'s in the tools directory"'
echo ""
echo "3. Natural language:"
echo '   "I want to see the test files"'
echo '   "Can you check what parser we have?"'
echo ""
echo "Starting Loco..."
echo "================"
./loco