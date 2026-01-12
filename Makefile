.PHONY: build clean install test release

# Build variables
BINARY_NAME=ralph
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Go commands
GO=go
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
GOGET=$(GO) get

# Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/ralph

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf dist/

# Install to GOPATH/bin
install:
	$(GO) install $(LDFLAGS) ./cmd/ralph

# Run tests
test:
	$(GOTEST) -v ./...

# Build for all platforms
build-all:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/ralph-darwin-arm64 ./cmd/ralph
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/ralph-darwin-amd64 ./cmd/ralph
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/ralph-linux-amd64 ./cmd/ralph
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/ralph-linux-arm64 ./cmd/ralph

# Release using goreleaser
release:
	goreleaser release --clean

# Snapshot release (no publish)
snapshot:
	goreleaser release --snapshot --clean

# Update dependencies
deps:
	$(GO) mod tidy

# Format code
fmt:
	$(GO) fmt ./...

# Lint code
lint:
	golangci-lint run
