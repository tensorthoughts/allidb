# AlliDB Documentation

Welcome to the AlliDB documentation. AlliDB is a distributed vector + graph database designed for RAG (Retrieval-Augmented Generation) workloads.

## Documentation Index

### Getting Started

- **[Architecture Overview](./ARCHITECTURE.md)** - High-level system architecture, components, and data flow
- **[API Documentation](./API.md)** - gRPC and REST API reference with examples
- **[Configuration Reference](./CONFIGURATION.md)** - Complete configuration guide with examples

### Deep Dives

- **[Usage Guide](./USAGE.md)** - Practical examples: vectorization, storage, search, graph queries
- **[Storage Internals](./STORAGE.md)** - File formats, write/read paths, compaction details
- **[Class Diagrams](./CLASS_DIAGRAMS.md)** - Detailed class structures and relationships
- **[Performance Metrics](./PERFORMANCE.md)** - Benchmarks, optimization guidelines, monitoring

### Architecture Decision Records

- **[ADR-001: Unified Entity](./adr/ADR-001-unified-entity.md)** - Entity model design
- **[ADR-002: LSM Storage](./adr/ADR-002-lsm-storage.md)** - Storage architecture
- **[ADR-003: Consistency Model](./adr/ADR-003-consistency-model.md)** - Consistency guarantees
- **[ADR-004: No Global Index](./adr/ADR-004-no-global-index.md)** - Indexing strategy
- **[ADR-005: LLM Integration](./adr/ADR-005-llm-integration.md)** - AI/LLM integration
- **[ADR-006: Configuration System](./adr/ADR-006-configuration-system.md)** - Configuration architecture
- **[ADR-007: Unified Index](./adr/ADR-007-unified-index.md)** - Vector + graph index design

## Quick Start

### Installation

```bash
# Build from source
make all

# Or use Docker
cd docker && docker-compose up
```

### Configuration

Create a config file at `configs/allidb.yaml`:

```yaml
node:
  id: "node-1"
  data_dir: "./data"

cluster:
  seed_nodes: ["node-1:7000"]
  replication_factor: 1

storage:
  wal:
    max_file_mb: 128
  memtable:
    max_mb: 256

index:
  hnsw:
    m: 16
    ef_construction: 200
    ef_search: 64

query:
  default_consistency: "ONE"
  timeout_ms: 5000
```

### Running

```bash
./build/allidbd -config configs/allidb.yaml
```

### Quick Example

```bash
# Health check
curl http://localhost:7001/health

# Put entity
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: your-api-key" \
  -d '{
    "entity": {
      "entity_id": "entity-1",
      "tenant_id": "tenant-1",
      "entity_type": "document",
      "embeddings": [{
        "version": 1,
        "vector": [0.1, 0.2, 0.3],
        "timestamp_nanos": 1234567890000000000
      }]
    }
  }'

# Query
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: your-api-key" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10
  }'
```

**For detailed usage examples, see [USAGE.md](./USAGE.md)**

## Key Features

- **Vector Search**: HNSW-based approximate nearest neighbor search
- **Graph Traversal**: Relationship-based candidate expansion
- **LSM-Tree Storage**: High-throughput write-optimized storage
- **Distributed**: Consistent hashing, gossip protocol, quorum replication
- **Multi-Tenant**: Built-in tenant isolation
- **Durable**: Write-ahead log with configurable fsync
- **Scalable**: Horizontal and vertical scaling support

## Architecture Highlights

```
Client → API (gRPC/REST) → Coordinator → Storage/Index → SSTable
                                                              ↓
                                                         Compaction
```

- **Write Path**: WAL → Memtable → SSTable
- **Read Path**: Memtable + SSTable → Unified Index → Results
- **Query Path**: ANN Search → Graph Expansion → Unified Scoring → Top-K

## Performance

- **Write Throughput**: ~100K writes/sec (single node)
- **Read Latency**: <10ms (p99, 1M vectors)
- **Query Throughput**: ~8K queries/sec (single node)
- **Recall@10**: >95% (1M vectors)

See [PERFORMANCE.md](./PERFORMANCE.md) for detailed benchmarks.

## Contributing

See the main project README for contribution guidelines.

## License

See the main project LICENSE file.

