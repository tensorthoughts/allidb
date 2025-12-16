# Makefile for AlliDB

# Variables
BINARY_NAME=allidbd
BUILD_DIR=build
CONFIG_DIR=$(BUILD_DIR)/configs
BINARY_PATH=$(BUILD_DIR)/$(BINARY_NAME)
MAIN_PACKAGE=./cmd/allidbd
GO_BUILD_TAGS=grpc
SAMPLE_CONFIG=$(CONFIG_DIR)/allidb.yaml

# Go build flags
LDFLAGS=-s -w
BUILD_FLAGS=-tags $(GO_BUILD_TAGS) -ldflags "$(LDFLAGS)"

.PHONY: all clean install deps build config help test ensure-build-dir bench

# Default target
all: deps build config

# Help target
help:
	@echo "AlliDB Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make all          - Install dependencies, build binary, and create config (default)"
	@echo "  make deps         - Download Go module dependencies"
	@echo "  make build        - Build the binary"
	@echo "  make config       - Create sample config file in build directory"
	@echo "  make install      - Install dependencies and build binary"
	@echo "  make clean        - Remove build directory and binary"
	@echo "  make test         - Run all tests"
	@echo "  make help         - Show this help message"
	@echo ""
	@echo "Build output:"
	@echo "  Binary: $(BINARY_PATH)"
	@echo "  Config: $(SAMPLE_CONFIG)"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed successfully"

# Create build directory
ensure-build-dir:
	@mkdir -p $(BUILD_DIR)
	@mkdir -p $(CONFIG_DIR)

# Build the binary
build: ensure-build-dir
	@echo "Building $(BINARY_NAME)..."
	@go build $(BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "Binary built successfully: $(BINARY_PATH)"

# Create sample config file
config: ensure-build-dir
	@echo "Creating sample config file..."
	@cp configs/allidb.yaml $(SAMPLE_CONFIG)
	@echo "Sample config created: $(SAMPLE_CONFIG)"
	@echo ""
	@echo "To run AlliDB:"
	@echo "  $(BINARY_PATH) -config $(SAMPLE_CONFIG)"

# Install dependencies and build (alias for all)
install: deps build config

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "Clean complete"
	@go clean -testcache

# Run tests
test:
	@echo "Running tests..."
	@go test -tags $(GO_BUILD_TAGS) ./... -v

# Run benchmarks (build-tagged)
bench:
	@echo "Running benchmarks..."
	@go run -tags write_benchmark ./benchmark/write_benchmark.go -duration=10s
	@go run -tags read_benchmark ./benchmark/read_benchmark.go -duration=10s
	@go run -tags mixed_benchmark ./benchmark/mixed_rag_benchmark.go -duration=10s

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -tags $(GO_BUILD_TAGS) ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install it from https://golangci-lint.run/"; \
	fi

# Generate protobuf files (if needed)
proto: ensure-build-dir
	@echo "Generating protobuf files..."
	@if [ -f api/grpc/allidb.proto ]; then \
		mkdir -p api/grpc/pb && \
		protoc --go_out=. --go_opt=paths=source_relative \
		       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		       api/grpc/allidb.proto && \
		mv api/grpc/*.pb.go api/grpc/pb/ 2>/dev/null || true && \
		echo "Protobuf files generated successfully"; \
	else \
		echo "No protobuf files found"; \
	fi

# Build for multiple platforms
build-all: deps ensure-build-dir
	@echo "Building for multiple platforms..."
	@GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	@GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	@echo "Multi-platform builds complete"

# Quick build (no deps, no config)
quick-build: ensure-build-dir
	@echo "Quick building $(BINARY_NAME)..."
	@go build $(BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "Binary built: $(BINARY_PATH)"

