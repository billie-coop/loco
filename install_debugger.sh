#!/bin/bash

# Install Delve debugger for Go
echo "Installing Delve debugger..."
go install github.com/go-delve/delve/cmd/dlv@latest

echo "Delve installed at: $(which dlv)"
echo ""
echo "VSCode debugging setup complete!"
echo ""
echo "To start debugging:"
echo "1. Open this project in VSCode"
echo "2. Install the Go extension if prompted"
echo "3. Open internal/tui/model.go"
echo "4. Click in the gutter next to line 334 to set a breakpoint"
echo "5. Press F5 to start debugging"
echo "6. Type 'hi' and press enter in the terminal"
echo ""
echo "See .vscode/debug_breakpoints.md for all recommended breakpoints!"