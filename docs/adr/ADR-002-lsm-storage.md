# ADR-002: LSM-Tree Storage Architecture

## Status
Accepted

## Context
AlliDB needs a storage engine that can:
- Handle high write throughput (vector embeddings, graph edges)
- Support efficient range queries and scans
- Provide durability guarantees
- Scale to large datasets
- Support compaction for space efficiency

Traditional B-tree indexes struggle with high write throughput due to random I/O. LSM-trees (Log-Structured Merge-trees) are designed for write-heavy workloads by batching writes and using sequential I/O.

## Decision
We will implement an **LSM-tree storage architecture** with:

### Components
1. **Write-Ahead Log (WAL)**: 
   - Append-only binary log
   - Configurable fsync (default: enabled, can be disabled for performance)
   - Single writer goroutine, multiple producers via channel
   - File rotation by size (default: 128MB, configurable)
   - CRC32 checksums per entry
   - Sequential writes for performance
   - WAL replay on startup for crash recovery

2. **Memtable**:
   - In-memory write buffer
   - Append-only semantics
   - Copy-on-write for lock-free reads
   - Size-based flush trigger
   - `map[EntityID][]Row` structure

3. **SSTable (Sorted String Table)**:
   - Immutable on-disk files
   - Binary format with header, data, index, footer
   - CRC32 per row for integrity
   - Index block for fast entity lookups
   - Mmap-friendly for efficient reads

4. **Compaction**:
   - Size-tiered compaction strategy
   - Async job scheduling with configurable concurrency
   - IO throttling (bandwidth and IOPS limits)
   - Stream merge with deduplication by timestamp
   - Atomic table swaps
   - Configurable size tier threshold (default: 4 files)
   - Min/max size and age-based selection
   - Background compaction loop with configurable check interval

## Consequences

### Positive
- **High write throughput**: Sequential writes to WAL and SSTables
- **Efficient reads**: Index blocks enable fast lookups
- **Durability**: WAL ensures no data loss
- **Space efficiency**: Compaction removes duplicates and old data
- **Scalability**: Can handle large datasets with many SSTables
- **Crash recovery**: WAL replay restores state

### Negative
- **Read amplification**: May need to check multiple SSTables
- **Write amplification**: Compaction rewrites data
- **Compaction overhead**: Background I/O can impact performance
- **Complexity**: More moving parts than simple B-tree

### Trade-offs
- Chose LSM-tree over B-tree for write performance
- Size-tiered over leveled compaction for simplicity
- Async compaction over synchronous for write latency
- IO throttling to prevent system overload

## Implementation Notes
- WAL: `core/storage/wal/` (wal.go, replay.go)
- Memtable: `core/storage/memtable/` (memtable.go)
- SSTable: `core/storage/sstable/` (format.go, writer.go, reader.go)
- Compaction: `core/storage/compaction/` (manager.go, planner.go, executor.go)
- All components use binary formats for efficiency
- Storage engine configuration: `core/storage/engine.go`
- Tenant isolation guard: `core/storage/guard.go`
- Configuration-driven: All parameters configurable via YAML
- WAL fsync can be disabled for higher throughput (trade-off: durability)

