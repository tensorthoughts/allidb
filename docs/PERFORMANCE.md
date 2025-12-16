# AlliDB Performance Metrics

## Table of Contents

1. [Performance Characteristics](#performance-characteristics)
2. [Benchmarks](#benchmarks)
3. [Optimization Guidelines](#optimization-guidelines)
4. [Monitoring](#monitoring)

---

## Performance Characteristics

### Write Performance

#### Single Node (No Replication)

| Metric | Value | Notes |
|--------|-------|-------|
| **Throughput** | ~100K writes/sec | Memtable writes only |
| **Latency (p50)** | <1ms | Memtable write |
| **Latency (p99)** | <5ms | Memtable write |
| **Latency (p99.9)** | <10ms | Memtable write |
| **WAL Write (no fsync)** | ~1-2μs | Per entry |
| **WAL Write (fsync)** | ~100-200μs | Per entry |
| **SSTable Flush** | ~10-50ms | 256MB memtable |

#### With Replication (RF=3, QUORUM)

| Metric | Value | Notes |
|--------|-------|-------|
| **Throughput** | ~30K writes/sec | Network limited |
| **Latency (p50)** | <5ms | Quorum write |
| **Latency (p99)** | <20ms | Quorum write |
| **Latency (p99.9)** | <50ms | Quorum write |

### Read Performance

#### Vector Search (ANN)

| Dataset Size | Latency (p50) | Latency (p99) | Recall@10 |
|--------------|---------------|---------------|-----------|
| 100K vectors | <1ms | <5ms | >95% |
| 1M vectors | <5ms | <15ms | >95% |
| 10M vectors | <10ms | <30ms | >90% |
| 100M vectors | <20ms | <60ms | >85% |

**Configuration**: M=16, ef_search=64, 3072 dimensions

#### Graph Traversal

| Hops | Latency (p50) | Latency (p99) | Notes |
|------|---------------|---------------|-------|
| 1 hop | <1ms | <3ms | Direct neighbors |
| 2 hops | <3ms | <10ms | 2-hop expansion |
| 3 hops | <5ms | <15ms | 3-hop expansion |

#### Entity Lookup

| Source | Latency (p50) | Latency (p99) |
|--------|---------------|---------------|
| Memtable | <1μs | <5μs |
| Entity Cache | <1μs | <5μs |
| SSTable (index) | ~10-50μs | ~100μs |
| SSTable (full scan) | ~1-10ms | ~50ms |

### Storage Performance

#### Compaction

| Operation | Throughput | Notes |
|-----------|------------|-------|
| Merge | ~100-500MB/s | Depends on I/O |
| Deduplication | ~1M rows/sec | CPU-bound |
| Write | ~200-1000MB/s | Sequential writes |

#### Space Efficiency

| Metric | Value | Notes |
|--------|-------|-------|
| **Raw Data** | 100% | Baseline |
| **With Indexes** | ~110% | Index overhead |
| **After Compaction** | ~70% | Deduplication |
| **WAL Overhead** | ~5% | Temporary |

---

## Benchmarks

We provide three runnable benchmarks under `benchmark/` (build-tagged to stay out of normal builds):

- `write_benchmark.go` (tag `write_benchmark`): write-only workload against a mock coordinator/ring. Flags: `-concurrency`, `-duration`, `-key-size`, `-value-size`, `-verbose`.
- `read_benchmark.go` (tag `read_benchmark`): read-only workload. Flags mirror the write benchmark.
- `mixed_rag_benchmark.go` (tag `mixed_benchmark`): mixed RAG-style workload that exercises vector + graph style access.

### Running benchmarks locally

From repo root:

- Write-only:  
  `go run -tags write_benchmark ./benchmark/write_benchmark.go -concurrency=32 -duration=30s -key-size=32 -value-size=1024`

- Read-only:  
  `go run -tags read_benchmark ./benchmark/read_benchmark.go -concurrency=64 -duration=30s -key-size=32`

- Mixed (RAG-like):  
  `go run -tags mixed_benchmark ./benchmark/mixed_rag_benchmark.go -concurrency=32 -duration=30s -vector-dim=1536 -graph-fanout=4`

All benchmarks print throughput, errors, and latency histograms via the built-in metrics registry. A sample text corpus is available at `benchmark/data/sample_text.txt` to seed embeddings and edges when experimenting locally.

---

## Optimization Guidelines

### Write Performance

#### 1. Disable WAL Fsync (Development Only)

```yaml
storage:
  wal:
    fsync: false  # 100x faster, but risk of data loss
```

**Impact**: ~100x faster writes, but data loss on crash

#### 2. Increase Memtable Size

```yaml
storage:
  memtable:
    max_mb: 1024  # Larger memtable = fewer flushes
```

**Impact**: Fewer SSTable flushes, more memory usage

#### 3. Batch Writes

Use `BatchPutEntities` instead of multiple `PutEntity` calls:

```go
// Bad: 100 individual writes
for _, entity := range entities {
    client.PutEntity(ctx, &pb.PutEntityRequest{Entity: entity})
}

// Good: 1 batch write
client.BatchPutEntities(ctx, &pb.BatchPutEntitiesRequest{
    Entities: entities,
})
```

**Impact**: 10-100x faster for bulk loads

#### 4. Increase Compaction Concurrency

```yaml
storage:
  compaction:
    max_concurrent: 8  # More parallel compactions
```

**Impact**: Faster compaction, more I/O

### Read Performance

#### 1. Increase HNSW Parameters

```yaml
index:
  hnsw:
    m: 32              # More connections = better recall
    ef_search: 128     # More candidates = better recall
```

**Impact**: Better recall, slower queries

#### 2. Increase Cache Sizes

```yaml
index:
  graph_cache_size: 50000   # More cached entities
  entity_cache_size: 5000   # More cached entities
```

**Impact**: Higher cache hit rate, more memory

#### 3. Use Consistency Level ONE

```yaml
query:
  default_consistency: "ONE"  # Fastest, least consistent
```

**Impact**: Faster reads, eventual consistency

#### 4. Reduce Graph Expansion

```yaml
query:
  max_graph_hops: 1  # Less graph traversal
```

**Impact**: Faster queries, less graph-based ranking

### Storage Performance

#### 1. Use Fast Storage

- **SSD**: Recommended for production
- **NVMe**: Best performance
- **HDD**: Not recommended (10x slower)

#### 2. Tune Compaction

```yaml
storage:
  compaction:
    max_iops: 5000           # Higher IOPS limit
    max_bandwidth_mbps: 500  # Higher bandwidth limit
```

**Impact**: Faster compaction, more I/O contention

#### 3. Reduce Size Tier Threshold

```yaml
storage:
  compaction:
    size_tier_threshold: 2  # More aggressive compaction
```

**Impact**: More compaction, better space efficiency

---

## Monitoring

### Key Metrics

#### Write Metrics

- **Write Throughput**: Writes per second
- **Write Latency**: p50, p99, p99.9
- **WAL Size**: Current WAL file size
- **Memtable Size**: Current memtable size
- **Flush Rate**: SSTable flushes per second

#### Read Metrics

- **Query Throughput**: Queries per second
- **Query Latency**: p50, p99, p99.9
- **Cache Hit Rate**: Entity cache hit rate
- **ANN Recall**: Recall@k for vector search
- **Graph Expansion**: Average hops per query

#### Storage Metrics

- **SSTable Count**: Number of SSTable files
- **SSTable Size**: Total SSTable size
- **Compaction Rate**: Compactions per hour
- **Space Usage**: Total disk usage
- **Space Efficiency**: Data size / disk size

#### Cluster Metrics

- **Replication Lag**: Time behind primary
- **Gossip Latency**: Gossip round-trip time
- **Node Health**: Node liveness status
- **Network I/O**: Bytes sent/received

### Prometheus Metrics (Future)

```prometheus
# Write metrics
allidb_writes_total{type="put_entity"}
allidb_write_latency_seconds{quantile="0.5"}
allidb_write_latency_seconds{quantile="0.99"}

# Read metrics
allidb_queries_total{type="query"}
allidb_query_latency_seconds{quantile="0.5"}
allidb_query_latency_seconds{quantile="0.99"}

# Storage metrics
allidb_sstable_count
allidb_sstable_size_bytes
allidb_memtable_size_bytes

# Cache metrics
allidb_cache_hits_total{cache="entity"}
allidb_cache_misses_total{cache="entity"}
```

---

## Performance Tuning Checklist

### For High Write Throughput

- [ ] Disable WAL fsync (if acceptable)
- [ ] Increase memtable size
- [ ] Use batch writes
- [ ] Increase compaction concurrency
- [ ] Use fast storage (SSD/NVMe)

### For Low Read Latency

- [ ] Increase cache sizes
- [ ] Use consistency level ONE
- [ ] Reduce graph expansion
- [ ] Tune HNSW parameters (ef_search)
- [ ] Use fast storage (SSD/NVMe)

### For High Space Efficiency

- [ ] Reduce size tier threshold
- [ ] Increase compaction frequency
- [ ] Tune compaction bandwidth
- [ ] Monitor SSTable count

### For Cluster Performance

- [ ] Optimize network (low latency)
- [ ] Use consistency level QUORUM (not ALL)
- [ ] Monitor replication lag
- [ ] Tune gossip interval
- [ ] Use load balancing

---

## Next Steps

- **Configuration**: See [CONFIGURATION.md](./CONFIGURATION.md)
- **Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md)
- **API**: See [API.md](./API.md)

