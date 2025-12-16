# Build stage
FROM golang:1.18-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make protobuf protobuf-dev

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Copy Makefile
COPY Makefile ./

# Install dependencies using Makefile
RUN make deps

# Copy source code
COPY . .

# Generate proto code using Makefile
RUN make proto

# Create config file using Makefile
RUN make config

# Build the application
# Using CGO_ENABLED=0 for static binary and Docker-optimized flags
RUN CGO_ENABLED=0 GOOS=linux go build -tags grpc -ldflags "-s -w" -a -installsuffix cgo -o build/allidbd ./cmd/allidbd

# Runtime stage
FROM alpine:latest

# Install ca-certificates for TLS and wget for healthchecks
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Create configs directory
RUN mkdir -p /app/configs

# Copy binary from builder
COPY --from=builder /build/build/allidbd .

# Copy config file from builder
COPY --from=builder /build/build/configs/allidb.yaml /app/configs/allidb.yaml

# Copy benchmark assets (optional local use)
COPY --from=builder /build/benchmark /app/benchmark

# Create data directories
RUN mkdir -p /data/wal /data/sstables /data/logs

# Expose ports
# 50051: gRPC
# 8080: HTTP/REST
EXPOSE 50051 8080

# Set environment variables
ENV ALLIDB_DATA_DIR=/data
ENV ALLIDB_WAL_DIR=/data/wal
ENV ALLIDB_SSTABLE_DIR=/data/sstables
ENV ALLIDB_GRPC_ADDR=0.0.0.0:50051
ENV ALLIDB_HTTP_ADDR=0.0.0.0:8080
ENV ALLIDB_LOG_LEVEL=INFO

# Run the application with config file
CMD ["./allidbd", "-config", "/app/configs/allidb.yaml"]

