.PHONY: build test lint clean install run dev help

# Binary name
BINARY := figma-query
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Default target
all: build

## build: Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/figma-query

## build-all: Build for multiple platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/figma-query
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/figma-query
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/figma-query
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/figma-query
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/figma-query

## test: Run tests
test:
	go test -v ./...

## test-short: Run tests (short mode)
test-short:
	go test -short ./...

## test-cover: Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linters
lint:
	go vet ./...
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## fmt: Format code
fmt:
	go fmt ./...
	gofmt -s -w .

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/
	rm -f coverage.out coverage.html

## install: Install binary to ~/.local/bin
install: build
	install -d ~/.local/bin
	install -m 755 $(BINARY) ~/.local/bin/$(BINARY)

## install-go: Install binary to GOPATH/bin
install-go:
	go install $(LDFLAGS) ./cmd/figma-query

## run: Run the server (requires FIGMA_ACCESS_TOKEN)
run: build
	./$(BINARY)

## dev: Run with go run for development
dev:
	go run ./cmd/figma-query

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

## update: Update dependencies
update:
	go get -u ./...
	go mod tidy

## pre-commit: Install pre-commit hooks
pre-commit:
	@which pre-commit > /dev/null || (echo "Install pre-commit: pip install pre-commit" && exit 1)
	pre-commit install

## pre-commit-run: Run pre-commit on all files
pre-commit-run:
	pre-commit run --all-files

## mcp-config: Print Claude Code MCP config
mcp-config:
	@echo '{'
	@echo '  "mcpServers": {'
	@echo '    "figma-query": {'
	@echo '      "command": "$(shell pwd)/$(BINARY)",'
	@echo '      "env": {'
	@echo '        "FIGMA_ACCESS_TOKEN": "YOUR_TOKEN_HERE"'
	@echo '      }'
	@echo '    }'
	@echo '  }'
	@echo '}'

## help: Show this help
help:
	@echo "figma-query - Token-efficient Figma MCP server"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
