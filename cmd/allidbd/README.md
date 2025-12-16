# AlliDB Server (allidbd)

The main server binary for AlliDB, a vector + graph database.

## Components Initialized

### ✅ Storage Layer
- **WAL (Write-Ahead Log)**: Append-only log for durability
- **Memtable**: In-memory write buffer with copy-on-write
- **Tenant Guard**: Enforces tenant isolation at storage layer

### ✅ Compaction
- **Compaction Manager**: Size-tiered compaction strategy
- **IO Throttling**: Prevents system overload
- **Async Jobs**: Background compaction processing

### ✅ Repair System
- **Repair Detector**: Detects replica divergence during reads
- **Repair Scheduler**: Queues and rate-limits repair tasks
- **Repair Executor**: Executes repair writes via coordinator
- **Entropy Repair**: Merkle tree-based repair (commented out, needs partition provider)

### ✅ Index & Query
- **Unified Index**: Combines HNSW vector search + graph expansion
- **Query Executor**: Executes queries with unified scoring
- **Hot Reload**: Automatically reloads SSTables

### ✅ Security
- **API Key Authentication**: HTTP and gRPC middleware
- **Tenant Isolation**: Enforced at storage layer
- **TLS Support**: Optional TLS/mTLS (requires cert files)

### ✅ Cluster Components
- **Consistent Hashing Ring**: Key-to-node routing
- **Gossip Protocol**: Node liveness and state exchange
- **Coordinator**: Quorum reads/writes, replica selection

### ✅ Observability
- **Structured Logging**: JSON logs with levels
- **Metrics**: Counter, Gauge, Histogram support
- **Tracing**: Distributed tracing with spans

## Configuration

### Environment Variables

- `ALLIDB_NODE_ID`: Node identifier (default: `node1`)
- `ALLIDB_GRPC_ADDR`: gRPC server address (default: `0.0.0.0:50051`)
- `ALLIDB_HTTP_ADDR`: HTTP server address (default: `0.0.0.0:8080`)
- `ALLIDB_DATA_DIR`: Data directory (default: `./data`)
- `ALLIDB_LOG_LEVEL`: Log level - DEBUG, INFO, WARN, ERROR (default: `INFO`)
- `ALLIDB_ENABLE_TLS`: Enable TLS (default: `false`)
- `ALLIDB_TLS_CERT`: TLS certificate file path
- `ALLIDB_TLS_KEY`: TLS key file path
- `ALLIDB_FAIL_HARD`: Fail hard on tenant violations (default: `true`)
- `ALLIDB_GOSSIP_ADDR`: Gossip protocol address (default: `0.0.0.0:7946`)

### Command Line Flags

All environment variables can also be set via command-line flags:

```bash
./allidbd \
  -node-id=node1 \
  -grpc-addr=0.0.0.0:50051 \
  -http-addr=0.0.0.0:8080 \
  -data-dir=./data \
  -log-level=INFO \
  -tls=false \
  -fail-hard=true
```

## Usage

### Local Development

```bash
# Build
go build -tags grpc -o allidbd ./cmd/allidbd

# Run
./allidbd

# With custom config
./allidbd -node-id=node1 -data-dir=/tmp/allidb -log-level=DEBUG
```

### Docker

```bash
# Build image
docker build -t allidb:latest .

# Run container
docker run -d \
  --name allidb \
  -p 50051:50051 \
  -p 8080:8080 \
  -v allidb-data:/data \
  -e ALLIDB_NODE_ID=node1 \
  allidb:latest
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    API Layer                            │
│  ┌──────────────┐              ┌──────────────┐        │
│  │  gRPC Server │              │  HTTP Gateway│        │
│  └──────┬───────┘              └──────┬───────┘        │
└─────────┼──────────────────────────────┼────────────────┘
          │                              │
          └──────────────┬───────────────┘
                         │
          ┌──────────────▼───────────────┐
          │      Coordinator             │
          │  (Quorum reads/writes)       │
          └──────────────┬───────────────┘
                         │
    ┌────────────────────┼────────────────────┐
    │                    │                    │
┌───▼────┐      ┌────────▼────────┐   ┌──────▼──────┐
│ Storage│      │  Query Executor │   │   Repair    │
│  Layer │      │  + Unified Index│   │   System    │
└───┬────┘      └─────────────────┘   └──────┬──────┘
    │                                         │
┌───▼────┐                              ┌────▼──────┐
│  WAL   │                              │ Detector  │
│ Memtable│                             │ Scheduler │
│ SSTable │                             │ Executor  │
└───┬────┘                              └───────────┘
    │
┌───▼────────┐
│ Compaction │
│  Manager   │
└────────────┘
```

## Completed Integrations

1. ✅ **Memtable wired into write path**: StorageService integrates WAL, memtable, and tenant guard
2. ✅ **Tenant guard wired into storage operations**: All storage operations validate tenant isolation
3. ✅ **Repair detector initialized**: Ready for use (requires coordinator modification for full integration)
4. ✅ **Entropy repair setup**: Partition provider, tree provider, comparator, executor, and orchestrator initialized
5. ✅ **TLS configuration**: Full TLS/mTLS support with certificate loading via security/tls package
6. ✅ **SSTable loading**: Unified index automatically loads SSTables on startup
7. ✅ **WAL replay**: WAL entries are replayed on startup to restore state

## Production-Ready Features

All enhancements have been implemented and are production-ready:

1. ✅ **Full repair detector integration**: Coordinator exposes `ReadWithResponses()` method, and `CoordinatorWithRepair` wrapper automatically detects divergence on all reads
2. ✅ **Complete memtable flushing**: Full SSTable writing implementation that:
   - Gets all entities from memtable using `GetAllEntities()` and `GetAllData()`
   - Writes entities sorted by entity ID to SSTable
   - Properly builds index and footer
   - Resets memtable only after successful flush
3. ✅ **Production partition provider**: Enumerates partitions based on ring nodes, with support for node-based partitioning
4. ✅ **Production tree provider**: Builds Merkle trees from actual partition data by:
   - Querying memtable and unified index for entities
   - Filtering entities by partition ownership (using ring)
   - Building Merkle trees from entity data
5. ✅ **Production data provider**: Fetches actual data from nodes by:
   - Querying local storage (memtable + unified index) for local nodes
   - Filtering by key range and partition ownership
   - Serializing entities to bytes for repair operations

## Architecture Improvements

### Repair Detection
- **Automatic**: All coordinator reads automatically check for divergence
- **Asynchronous**: Repairs are scheduled and executed asynchronously
- **Non-blocking**: Read operations continue even if divergence is detected

### Memtable Flushing
- **Complete**: All entities are written to SSTable before reset
- **Sorted**: Entities are written in sorted order for optimal read performance
- **Atomic**: Memtable is only reset after successful flush

### Entropy Repair
- **Partition-based**: Partitions are derived from ring topology
- **Merkle trees**: Built from actual entity data
- **Range-based**: Repairs operate on key ranges for efficiency
- **Periodic**: Automatic periodic repairs via orchestrator
8. **Metrics export**: Export metrics to Prometheus/other systems
9. **Tracing export**: Export traces to Jaeger/other systems

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./core/storage/compaction
```

