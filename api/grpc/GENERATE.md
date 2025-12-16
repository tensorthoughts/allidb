# Generating Proto Code

## Step 1: Install Required Dependencies

```bash
# Install protobuf compiler plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add Go bin to PATH if not already there (usually ~/go/bin)
export PATH=$PATH:$(go env GOPATH)/bin
```

## Step 2: Generate Proto Code

From the project root (`projects/allidb`):

```bash
# Create pb directory first
mkdir -p api/grpc/pb

# Generate proto code (files will be created in api/grpc/, then move to pb/)
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/grpc/allidb.proto

# Move generated files to pb directory
mv api/grpc/*.pb.go api/grpc/pb/
```

Or as a one-liner:

```bash
mkdir -p api/grpc/pb && \
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/grpc/allidb.proto && \
mv api/grpc/*.pb.go api/grpc/pb/
```

This will generate:
- `api/grpc/pb/allidb.pb.go` - Message types
- `api/grpc/pb/allidb_grpc.pb.go` - Service interface

## Step 3: Update go.mod

After generation, you may need to add the gRPC dependencies:

```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
go mod tidy
```

