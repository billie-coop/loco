.PHONY: help run test watch coverage progress next clean install-tools

# Default target
help:
	@echo "Loco Development Commands:"
	@echo "  make run          - Run the application"
	@echo "  make test         - Run all tests"
	@echo "  make watch        - Run tests in watch mode (TDD)"
	@echo "  make coverage     - Generate test coverage report"
	@echo "  make progress     - Show roadmap progress"
	@echo "  make next         - Show next test to implement"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install-tools - Install development tools"

# Run the application (fresh start - deletes .loco directory)
run:
	rm -rf .loco
	go run .

# Run all tests
test:
	go test -v ./...

# TDD watch mode - automatically runs tests on file changes
watch:
	@if command -v gotestsum >/dev/null 2>&1; then \
		gotestsum --watch; \
	else \
		echo "Installing gotestsum for watch mode..."; \
		go install gotest.tools/gotestsum@latest; \
		gotestsum --watch; \
	fi

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Show roadmap progress
progress:
	@echo "=== Loco Development Progress ==="
	@echo -n "📝 TODO: "
	@go test -v ./... 2>&1 | grep -c "SKIP" || echo "0"
	@echo -n "✅ DONE: "
	@go test -v ./... 2>&1 | grep -c "PASS" || echo "0"
	@echo "================================"

# Show next skipped test to work on
next:
	@echo "Next test to implement:"
	@go test -v ./... 2>&1 | grep "SKIP" | head -3

# Clean build artifacts
clean:
	rm -f coverage.out coverage.html
	rm -f loco

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install gotest.tools/gotestsum@latest
	go install github.com/vektra/mockery/v2@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "Installing pre-commit..."
	pip install --user pre-commit || pip3 install --user pre-commit
	pre-commit install
	@echo "All tools installed!"

# Linting commands
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

# Security check
security:
	gosec -severity high ./...

# Pre-commit manually
pre-commit:
	pre-commit run --all-files

# Build binary
build:
	go build -o loco .

# Run with debug output
debug:
	LOCO_DEBUG=true go run .