# AlliDB gRPC API

This package implements the gRPC API for AlliDB.

## Proto Definition

The proto definition is in `allidb.proto`. To generate Go code from the proto file:

```bash
# Install required tools
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate code
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/grpc/allidb.proto
```

This will generate:
- `api/grpc/pb/allidb.pb.go` - Message types
- `api/grpc/pb/allidb_grpc.pb.go` - Service interface

## Server Implementation

The server implementation in `server.go` provides:

1. **Authentication**: Uses gRPC auth interceptor to extract tenant ID from API key
2. **Validation**: Validates tenant ID matches entity tenant ID
3. **Coordinator Integration**: Forwards write operations to coordinator with configurable consistency levels
4. **Query Execution**: Executes queries using the query executor

## Usage

Once the proto code is generated, update `server.go` to:

1. Import the generated proto package:
   ```go
   import pb "github.com/tensorthoughts25/allidb/api/grpc/pb"
   ```

2. Update method signatures to use proto types:
   ```go
   func (s *Server) PutEntity(ctx context.Context, req *pb.PutEntityRequest) (*pb.PutEntityResponse, error)
   ```

3. Uncomment the implementation code in each method

4. Register the server with gRPC:
   ```go
   import (
       "google.golang.org/grpc"
       "github.com/tensorthoughts25/allidb/api/grpc"
       pb "github.com/tensorthoughts25/allidb/api/grpc/pb"
   )
   
   s := grpc.NewServer(
       grpc.UnaryInterceptor(auth.GRPCAuthInterceptor(apiKeyStore)),
   )
   
   allidbServer := grpc.NewServer(grpc.Config{
       Coordinator:   coordinator,
       QueryExecutor: queryExecutor,
   })
   
   pb.RegisterAlliDBServer(s, allidbServer)
   ```

## TLS and Remote Calls

- The server supports TLS if configured in `security.tls.*` (see `cmd/allidbd/main.go`).
- Remote range-scan forwarding currently defaults to `grpc.WithInsecure()`; enable TLS for production by configuring TLS and updating the dial options.
- Ensure nodes publish reachable addresses via gossip so remote calls can resolve correctly.

## Consistency Levels

The API supports three consistency levels:

- **ONE**: Write to/read from one replica (fastest, least consistent)
- **QUORUM**: Write to/read from a quorum of replicas (balanced)
- **ALL**: Write to/read from all replicas (slowest, most consistent)

## Trace ID

All requests can include an optional `trace_id` for distributed tracing. The trace ID is echoed in responses.

