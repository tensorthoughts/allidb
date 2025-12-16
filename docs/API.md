# AlliDB API Documentation

## Table of Contents

1. [gRPC API](#grpc-api)
2. [REST API](#rest-api)
3. [Authentication](#authentication)
4. [Examples](#examples)
5. [Error Handling](#error-handling)

---

## gRPC API

### Service Definition

AlliDB exposes a gRPC service with the following methods:

```protobuf
service AlliDB {
  rpc PutEntity(PutEntityRequest) returns (PutEntityResponse);
  rpc BatchPutEntities(BatchPutEntitiesRequest) returns (BatchPutEntitiesResponse);
  rpc Query(QueryRequest) returns (QueryResponse);
  rpc RangeScan(RangeScanRequest) returns (RangeScanResponse);
  rpc ReplicaRead(ReplicaReadRequest) returns (ReplicaReadResponse);
  rpc ReplicaWrite(ReplicaWriteRequest) returns (ReplicaWriteResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}
```

### Endpoints

**Default Port**: `7000` (configurable via `node.grpc_port`)

**Connection**: `grpc://<host>:<port>`

---

## Methods

### PutEntity

Store a single entity.

**Request**:
```protobuf
message PutEntityRequest {
  Entity entity = 1;
  ConsistencyLevel consistency_level = 2;  // ONE, QUORUM, ALL
  string trace_id = 3;  // Optional trace ID
}
```

**Response**:
```protobuf
message PutEntityResponse {
  bool success = 1;
  string error_message = 2;
  string entity_id = 3;
}
```

**Example (Go)**:
```go
import (
    "context"
    pb "github.com/tensorthoughts25/allidb/api/grpc/pb"
    "google.golang.org/grpc"
)

conn, _ := grpc.Dial("localhost:7000", grpc.WithInsecure())
client := pb.NewAlliDBClient(conn)

req := &pb.PutEntityRequest{
    Entity: &pb.Entity{
        EntityId:   "entity-123",
        TenantId:   "tenant-1",
        EntityType: "document",
        Embeddings: []*pb.VectorEmbedding{
            {
                Version:  1,
                Vector:   []float32{0.1, 0.2, 0.3, ...},
                TimestampNanos: time.Now().UnixNano(),
            },
        },
        Importance: 0.8,
    },
    ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
    TraceId: "trace-abc123",
}

resp, err := client.PutEntity(context.Background(), req)
```

**Example (Python)**:
```python
import grpc
from allidb_pb2 import PutEntityRequest, Entity, VectorEmbedding, ConsistencyLevel
from allidb_pb2_grpc import AlliDBStub

channel = grpc.insecure_channel('localhost:7000')
stub = AlliDBStub(channel)

request = PutEntityRequest(
    entity=Entity(
        entity_id="entity-123",
        tenant_id="tenant-1",
        entity_type="document",
        embeddings=[
            VectorEmbedding(
                version=1,
                vector=[0.1, 0.2, 0.3],
                timestamp_nanos=int(time.time() * 1e9)
            )
        ],
        importance=0.8
    ),
    consistency_level=ConsistencyLevel.CONSISTENCY_QUORUM,
    trace_id="trace-abc123"
)

response = stub.PutEntity(request)
```

**Example (cURL via grpcurl)**:
```bash
grpcurl -plaintext -d '{
  "entity": {
    "entity_id": "entity-123",
    "tenant_id": "tenant-1",
    "entity_type": "document",
    "embeddings": [{
      "version": 1,
      "vector": [0.1, 0.2, 0.3],
      "timestamp_nanos": 1234567890000000000
    }],
    "importance": 0.8
  },
  "consistency_level": "CONSISTENCY_QUORUM"
}' localhost:7000 allidb.AlliDB/PutEntity
```

---

### BatchPutEntities

Store multiple entities in a single request.

**Request**:
```protobuf
message BatchPutEntitiesRequest {
  repeated Entity entities = 1;
  ConsistencyLevel consistency_level = 2;
  string trace_id = 3;
}
```

**Response**:
```protobuf
message BatchPutEntitiesResponse {
  bool success = 1;
  string error_message = 2;
  repeated string entity_ids = 3;
  int32 success_count = 4;
  int32 failure_count = 5;
}
```

**Example (Go)**:
```go
req := &pb.BatchPutEntitiesRequest{
    Entities: []*pb.Entity{
        {EntityId: "entity-1", TenantId: "tenant-1", ...},
        {EntityId: "entity-2", TenantId: "tenant-1", ...},
        {EntityId: "entity-3", TenantId: "tenant-1", ...},
    },
    ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
}

resp, err := client.BatchPutEntities(context.Background(), req)
fmt.Printf("Success: %d, Failed: %d\n", resp.SuccessCount, resp.FailureCount)
```

---

### Query

Perform a vector + graph search query.

**Request**:
```protobuf
message QueryRequest {
  repeated float query_vector = 1;  // Query vector (float32)
  int32 k = 2;  // Number of results (default: 10)
  int32 expand_factor = 3;  // Graph expansion factor (default: 2)
  string trace_id = 4;
  ConsistencyLevel consistency_level = 5;
}
```

**Response**:
```protobuf
message QueryResponse {
  repeated QueryResult results = 1;
  string trace_id = 2;
  int32 total_results = 3;
}

message QueryResult {
  string entity_id = 1;
  double score = 2;
  Entity entity = 3;  // Optional, may be omitted
}
```

**Example (Go)**:
```go
req := &pb.QueryRequest{
    QueryVector: []float32{0.1, 0.2, 0.3, ...},  // 3072 dimensions
    K: 10,
    ExpandFactor: 2,
    ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
}

resp, err := client.Query(context.Background(), req)
for _, result := range resp.Results {
    fmt.Printf("Entity: %s, Score: %.4f\n", result.EntityId, result.Score)
}
```

**Example (Python)**:
```python
request = QueryRequest(
    query_vector=[0.1, 0.2, 0.3] * 1024,  # 3072 dimensions
    k=10,
    expand_factor=2,
    consistency_level=ConsistencyLevel.CONSISTENCY_QUORUM
)

response = stub.Query(request)
for result in response.results:
    print(f"Entity: {result.entity_id}, Score: {result.score}")
```

---

### RangeScan

Scan entities within a key range. Useful for pagination and bulk operations.

**Request**:
```protobuf
message RangeScanRequest {
  string tenant_id = 1;     // Tenant identifier (optional if provided via auth)
  string start_key = 2;     // Inclusive start key (empty = beginning)
  string end_key = 3;       // Exclusive end key (empty = no upper bound)
  int32 limit = 4;          // Maximum entities to return (pagination)
  string page_token = 5;    // Opaque page token (offset-based)
  string node_id = 6;       // Optional target node (if empty, server routes locally)
}
```

**Response**:
```protobuf
message RangeScanResponse {
  repeated Entity entities = 1;   // Entities in sorted key order
  string next_page_token = 2;     // Next page token if more results
}
```

**Example (Go)**:
```go
req := &pb.RangeScanRequest{
    TenantId: "tenant-1",
    StartKey: "entity-100",
    EndKey:   "entity-200",
    Limit:    100,
}

resp, err := client.RangeScan(context.Background(), req)
for _, entity := range resp.Entities {
    fmt.Printf("Entity: %s\n", entity.EntityId)
}

// Pagination
if resp.NextPageToken != "" {
    req.PageToken = resp.NextPageToken
    resp, err = client.RangeScan(context.Background(), req)
}
```

**Note**: RangeScan is primarily for internal use and administrative operations. For production queries, use the Query endpoint.

---

### ReplicaRead / ReplicaWrite

Internal replication endpoints used by the coordinator for quorum-based replication. These are not typically called directly by clients.

**ReplicaRead**:
```protobuf
message ReplicaReadRequest {
  string key = 1;
  string tenant_id = 2;
}

message ReplicaReadResponse {
  bytes value = 1;
}
```

**ReplicaWrite**:
```protobuf
message ReplicaWriteRequest {
  string key = 1;
  bytes value = 2;
  string tenant_id = 3;
}

message ReplicaWriteResponse {
  bool success = 1;
}
```

**Note**: These endpoints are used internally by AlliDB's replication system. Direct client usage is not recommended.

---

### HealthCheck

Check the health of the service.

**Request**:
```protobuf
message HealthCheckRequest {
  // Empty
}
```

**Response**:
```protobuf
message HealthCheckResponse {
  bool healthy = 1;
  string status = 2;
  int64 timestamp_nanos = 3;
}
```

**Example (Go)**:
```go
resp, err := client.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
if resp.Healthy {
    fmt.Println("Service is healthy")
}
```

---

## REST API

### Endpoints

**Base URL**: `http://<host>:<port>` (default: `http://localhost:7001`)

**Authentication**: Header `X-ALLIDB-API-KEY: <api-key>`

### POST /entity

Store a single entity.

**Request**:
```http
POST /entity HTTP/1.1
Host: localhost:7001
Content-Type: application/json
X-ALLIDB-API-KEY: your-api-key

{
  "entity": {
    "entity_id": "entity-123",
    "tenant_id": "tenant-1",
    "entity_type": "document",
    "embeddings": [{
      "version": 1,
      "vector": [0.1, 0.2, 0.3],
      "timestamp_nanos": 1234567890000000000
    }],
    "importance": 0.8
  },
  "consistency_level": "CONSISTENCY_QUORUM",
  "trace_id": "trace-abc123"
}
```

**Response**:
```json
{
  "success": true,
  "entity_id": "entity-123"
}
```

**Example (cURL)**:
```bash
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: your-api-key" \
  -d '{
    "entity": {
      "entity_id": "entity-123",
      "tenant_id": "tenant-1",
      "entity_type": "document",
      "embeddings": [{
        "version": 1,
        "vector": [0.1, 0.2, 0.3],
        "timestamp_nanos": 1234567890000000000
      }]
    },
    "consistency_level": "CONSISTENCY_QUORUM"
  }'
```

---

### POST /entities/batch

Store multiple entities.

**Request**:
```http
POST /entities/batch HTTP/1.1
Host: localhost:7001
Content-Type: application/json
X-ALLIDB-API-KEY: your-api-key

{
  "entities": [
    {
      "entity_id": "entity-1",
      "tenant_id": "tenant-1",
      ...
    },
    {
      "entity_id": "entity-2",
      "tenant_id": "tenant-1",
      ...
    }
  ],
  "consistency_level": "CONSISTENCY_QUORUM"
}
```

**Response**:
```json
{
  "success": true,
  "entity_ids": ["entity-1", "entity-2"],
  "success_count": 2,
  "failure_count": 0
}
```

---

### POST /query

Perform a vector + graph search query.

**Request**:
```http
POST /query HTTP/1.1
Host: localhost:7001
Content-Type: application/json
X-ALLIDB-API-KEY: your-api-key

{
  "query_vector": [0.1, 0.2, 0.3, ...],
  "k": 10,
  "expand_factor": 2,
  "consistency_level": "CONSISTENCY_QUORUM"
}
```

**Response**:
```json
{
  "results": [
    {
      "entity_id": "entity-123",
      "score": 0.95,
      "entity": {
        "entity_id": "entity-123",
        "tenant_id": "tenant-1",
        ...
      }
    },
    ...
  ],
  "total_results": 10
}
```

**Example (cURL)**:
```bash
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: your-api-key" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "expand_factor": 2
  }'
```

---

### GET /health

Check service health.

**Request**:
```http
GET /health HTTP/1.1
Host: localhost:7001
```

**Response**:
```json
{
  "healthy": true,
  "status": "ok",
  "timestamp_nanos": 1234567890000000000
}
```

**Example (cURL)**:
```bash
curl http://localhost:7001/health
```

---

## Authentication

### API Key Authentication

All requests (except `/health`) require an API key in the header:

**gRPC**: Metadata key `x-allidb-api-key`

**REST**: Header `X-ALLIDB-API-KEY`

**Example (Go gRPC)**:
```go
md := metadata.New(map[string]string{
    "x-allidb-api-key": "your-api-key",
})
ctx := metadata.NewOutgoingContext(context.Background(), md)
resp, err := client.PutEntity(ctx, req)
```

**Example (Python gRPC)**:
```python
metadata = [('x-allidb-api-key', 'your-api-key')]
response = stub.PutEntity(request, metadata=metadata)
```

---

## Consistency Levels

- **ONE**: Write to/read from one replica (fastest, least consistent)
- **QUORUM**: Write to/read from majority of replicas (balanced)
- **ALL**: Write to/read from all replicas (slowest, most consistent)

**Default**: `QUORUM`

---

## Error Handling

### gRPC Errors

gRPC uses standard status codes:

- `OK` (0): Success
- `INVALID_ARGUMENT` (3): Invalid request
- `UNAUTHENTICATED` (16): Missing or invalid API key
- `PERMISSION_DENIED` (7): Tenant isolation violation
- `INTERNAL` (13): Server error
- `DEADLINE_EXCEEDED` (4): Request timeout

**Example (Go)**:
```go
resp, err := client.PutEntity(ctx, req)
if err != nil {
    if st, ok := status.FromError(err); ok {
        switch st.Code() {
        case codes.UNAUTHENTICATED:
            fmt.Println("Authentication failed")
        case codes.DEADLINE_EXCEEDED:
            fmt.Println("Request timeout")
        default:
            fmt.Printf("Error: %v\n", err)
        }
    }
}
```

### REST Errors

REST uses HTTP status codes:

- `200 OK`: Success
- `400 Bad Request`: Invalid request
- `401 Unauthorized`: Missing or invalid API key
- `403 Forbidden`: Tenant isolation violation
- `500 Internal Server Error`: Server error
- `504 Gateway Timeout`: Request timeout

**Error Response Format**:
```json
{
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

---

## Rate Limiting

Currently not implemented. Future versions may include:
- Per-tenant rate limits
- Per-API-key rate limits
- Global rate limits

---

## Timeouts

**Default Timeout**: 5 seconds (configurable via `query.timeout_ms`)

**Client-Side Timeout** (Go):
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
resp, err := client.Query(ctx, req)
```

---

## Next Steps

- **Configuration**: See [CONFIGURATION.md](./CONFIGURATION.md)
- **Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md)
- **Performance**: See [PERFORMANCE.md](./PERFORMANCE.md)

