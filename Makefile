.PHONY: all build snapshot test test-coverage lint fmt vet tidy deps clean install run version tag help

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

# Build every release platform locally via GoReleaser (no publishing)
snapshot:
	goreleaser release --snapshot --clean --skip=docker,publish

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
	@echo "The Release workflow then publishes binaries, the image and the Homebrew cask."

# Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Version: $(VERSION) (Git commit: $(GIT_COMMIT))"
	@echo ""
	@echo "Targets:"
	@echo "  all           -- Run test and build (default)"
	@echo "  build         -- Build the binary to dist/"
	@echo "  snapshot      -- Build all release artifacts locally (GoReleaser)"
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
	@echo "  help          -- Show this help"
