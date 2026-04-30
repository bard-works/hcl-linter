.PHONY: all build test lint clean install run help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary name
BINARY_NAME=hcl-linter

# Version info
VERSION=$(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_TREE_STATE=$(shell if git status --porcelain 2>/dev/null | grep -q .; then echo "dirty"; else echo "clean"; fi)

# LDFLAGS for version injection
LDFLAGS=-ldflags "\
  -s -w \
  -X main.Version=$(VERSION) \
  -X main.BuildDate=$(BUILD_DATE) \
  -X main.GitCommit=$(GIT_COMMIT)"

# Directories
DIST_DIR=dist
CMD_DIR=cmd/hcl-linter

# Default target
all: test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(DIST_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# Build for multiple platforms
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64

build-darwin-amd64:
	@echo "Building $(BINARY_NAME)-darwin-amd64..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)

build-darwin-arm64:
	@echo "Building $(BINARY_NAME)-darwin-arm64..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)

build-linux-amd64:
	@echo "Building $(BINARY_NAME)-linux-amd64..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)

build-linux-arm64:
	@echo "Building $(BINARY_NAME)-linux-arm64..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

build-windows-amd64:
	@echo "Building $(BINARY_NAME)-windows-amd64.exe..."
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Vet code
vet:
	@echo "Vetting code..."
	$(GOVET) ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(DIST_DIR)
	@rm -f coverage.out coverage.html

# Install binary to GOPATH
install:
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) ./$(CMD_DIR)

# Run the binary
run:
	@./$(DIST_DIR)/$(BINARY_NAME)

# Show version
version:
	@echo "Version: $(VERSION)"
	@echo "Build date: $(BUILD_DATE)"
	@echo "Git commit: $(GIT_COMMIT)"
	@echo "Git tree state: $(GIT_TREE_STATE)"

# Tag a new release
tag:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make tag VERSION=x.x.x"; \
		exit 1; \
	fi
	@echo "Tagging v$(VERSION)..."
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@echo "Tagged v$(VERSION). Push with: git push origin v$(VERSION)"

# Release: tag, build all, and create checksums
release: tag build-all checksums

# Generate checksums for all binaries
checksums:
	@echo "Generating checksums..."
	@cd $(DIST_DIR) && \
	for f in $(BINARY_NAME)*; do \
		sha256sum $$f > $$f.sha256; \
	done
	@ls -la $(DIST_DIR)

# Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Version: $(VERSION) (Git commit: $(GIT_COMMIT))"
	@echo ""
	@echo "Targets:"
	@echo "  all           -- Run test and build (default)"
	@echo "  build         -- Build the binary to dist/"
	@echo "  build-all     -- Build for all platforms"
	@echo "  test          -- Run tests"
	@echo "  test-coverage -- Run tests with coverage report"
	@echo "  lint          -- Run golangci-lint"
	@echo "  fmt           -- Format code"
	@echo "  vet           -- Vet code"
	@echo "  tidy          -- Tidy dependencies"
	@echo "  deps          -- Download dependencies"
	@echo "  clean         -- Remove build artifacts"
	@echo "  install       -- Install binary to GOPATH/bin"
	@echo "  run           -- Run the built binary"
	@echo "  version       -- Show version info"
	@echo "  tag           -- Tag a release (VERSION=x.x.x make tag)"
	@echo "  release       -- Tag, build all, and generate checksums"
	@echo "  help          -- Show this help"
