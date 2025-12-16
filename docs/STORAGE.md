# AlliDB Storage Internals

## Table of Contents

1. [Storage Architecture](#storage-architecture)
2. [File Formats](#file-formats)
3. [Write Path](#write-path)
4. [Read Path](#read-path)
5. [Compaction](#compaction)
6. [Data Structures](#data-structures)

---

## Storage Architecture

AlliDB uses an LSM-Tree (Log-Structured Merge-tree) storage architecture optimized for high write throughput and efficient reads.

### Component Overview

```
┌─────────────────────────────────────────┐
│           Storage Engine                │
├─────────────────────────────────────────┤
│                                         │
│  ┌──────────┐      ┌──────────┐       │
│  │   WAL    │─────▶│ Memtable │       │
│  │ (Durable)│      │ (Memory) │       │
│  └────┬─────┘      └────┬─────┘       │
│       │                 │              │
│       │                 │ (Flush)      │
│       │                 ▼              │
│       │          ┌──────────┐         │
│       │          │ SSTable  │         │
│       │          │  (Disk)  │         │
│       │          └────┬─────┘         │
│       │               │                │
│       │               │ (Compact)     │
│       │               ▼                │
│       │          ┌──────────┐         │
│       └─────────▶│Compaction│         │
│                  │ Manager  │         │
│                  └──────────┘         │
└─────────────────────────────────────────┘
```

---

## File Formats

### WAL (Write-Ahead Log) Format

**File Naming**: `wal-{sequence}.wal` (e.g., `wal-00000001.wal`)

**File Structure**:
```
[Entry 1]
  [4 bytes: Entry Length (uint32, little-endian)]
  [4 bytes: CRC32 Checksum (uint32, little-endian)]
  [N bytes: Entry Data (serialized WALEntry)]
[Entry 2]
  ...
[Entry N]
```

**Entry Data Format** (for EntityWALEntry):
```
[4 bytes: EntityID Length (uint32)]
[N bytes: EntityID (UTF-8 string)]
[4 bytes: TenantID Length (uint32)]
[M bytes: TenantID (UTF-8 string)]
[8 bytes: Timestamp (int64, nanoseconds since epoch)]
[4 bytes: Data Length (uint32)]
[K bytes: Serialized Entity Data (binary)]
```

**Example**:
```
Offset 0x0000: [00 00 00 20]  Entry Length: 32 bytes
Offset 0x0004: [A1 B2 C3 D4]  CRC32 Checksum
Offset 0x0008: [00 00 00 05]  EntityID Length: 5
Offset 0x000C: [65 6E 74 31 32]  EntityID: "ent12"
...
```

**File Rotation**:
- Triggered when file size exceeds `max_file_mb` (default: 128MB)
- New file sequence number increments
- Old file remains for replay

### SSTable Format

**File Naming**: `sstable-{sequence}.sst` (e.g., `sstable-00000001.sst`)

**File Structure**:
```
┌─────────────────────────────────────┐
│ Header (16 bytes)                   │
├─────────────────────────────────────┤
│ Data Section (variable)             │
│   [Row 1]                           │
│   [Row 2]                           │
│   ...                               │
│   [Row N]                           │
├─────────────────────────────────────┤
│ Index Section (variable)            │
│   [Index Entry 1]                   │
│   [Index Entry 2]                   │
│   ...                               │
│   [Index Entry N]                   │
├─────────────────────────────────────┤
│ Footer (24 bytes)                   │
└─────────────────────────────────────┘
```

#### Header (16 bytes)

```
Offset 0x0000: [4 bytes] Magic Number (0x53535442 = "SSTB")
Offset 0x0004: [2 bytes] Version (1)
Offset 0x0006: [2 bytes] Reserved (padding)
Offset 0x0008: [8 bytes] DataOffset (int64, offset to data section)
```

#### Row Format

**Row Types**:
- `0x01`: VECTOR - Vector embedding row
- `0x02`: EDGE - Graph edge row
- `0x03`: CHUNK - Text chunk row
- `0x04`: META - Entity metadata row

**Row Layout**:
```
[1 byte]  Row Type
[4 bytes] EntityID Length (uint32)
[N bytes] EntityID (UTF-8 string)
[4 bytes] Data Length (uint32)
[M bytes] Row Data (type-specific)
[4 bytes] CRC32 Checksum (uint32)
```

**VECTOR Row Data**:
```
[8 bytes]  Version (int64)
[4 bytes]  Vector Dimension (uint32)
[N*4 bytes] Vector Data (float32 array, little-endian)
[8 bytes]  Timestamp (int64, nanoseconds)
```

**EDGE Row Data**:
```
[4 bytes]  FromEntityID Length (uint32)
[N bytes]  FromEntityID (UTF-8 string)
[4 bytes]  ToEntityID Length (uint32)
[M bytes]  ToEntityID (UTF-8 string)
[4 bytes]  Relation Length (uint32)
[K bytes]  Relation (UTF-8 string)
[8 bytes]  Weight (float64, little-endian)
[8 bytes]  Timestamp (int64, nanoseconds)
```

**CHUNK Row Data**:
```
[4 bytes]  ChunkID Length (uint32)
[N bytes]  ChunkID (UTF-8 string)
[4 bytes]  Text Length (uint32)
[M bytes]  Text (UTF-8 string)
[4 bytes]  Metadata Count (uint32)
[For each metadata entry:]
  [4 bytes]  Key Length (uint32)
  [K bytes]  Key (UTF-8 string)
  [4 bytes]  Value Length (uint32)
  [V bytes]  Value (UTF-8 string)
```

**META Row Data**:
```
[4 bytes]  EntityType Length (uint32)
[N bytes]  EntityType (UTF-8 string)
[8 bytes]  Importance (float64, little-endian)
[8 bytes]  CreatedAt (int64, nanoseconds)
[8 bytes]  UpdatedAt (int64, nanoseconds)
```

#### Index Section

**Index Entry Format**:
```
[4 bytes]  EntityID Length (uint32)
[N bytes]  EntityID (UTF-8 string)
[8 bytes]  Offset (int64, offset to first row for this entity)
```

**Index Organization**:
- Sorted by EntityID (lexicographic order)
- Binary search for O(log n) lookups
- All rows for an entity are contiguous

#### Footer (24 bytes)

```
Offset 0x0000: [8 bytes] IndexOffset (int64, offset to index section)
Offset 0x0008: [8 bytes] IndexSize (int64, size of index section)
Offset 0x0010: [8 bytes] RowCount (int64, total number of rows)
```

**Example SSTable Layout**:
```
0x0000: [53 53 54 42]  Magic: "SSTB"
0x0004: [00 01]        Version: 1
0x0006: [00 00]        Reserved
0x0008: [00 00 00 00 00 00 00 10]  DataOffset: 16
...
0x0010: [Row 1 starts here]
...
0x1000: [Index section starts here]
...
0x2000: [Footer starts here]
  0x2000: [00 00 00 00 00 00 10 00]  IndexOffset: 4096
  0x2008: [00 00 00 00 00 00 08 00]  IndexSize: 2048
  0x2010: [00 00 00 00 00 00 00 64]  RowCount: 100
```

---

## Write Path

### Step-by-Step Write Flow

```
1. Client Write Request
   ↓
2. StorageService.PutEntity()
   ├─→ Tenant Guard (Validate tenant isolation)
   ├─→ Serialize Entity to WALEntry
   └─→ WAL.Append(entry)
       ├─→ Channel send (async)
       └─→ Writer goroutine:
           ├─→ Write entry to WAL file
           ├─→ Calculate CRC32
           ├─→ Fsync (if enabled)
           └─→ Acknowledge
   ↓
3. Memtable.Put(row)
   ├─→ Acquire write lock
   ├─→ Append row to map[EntityID][]Row
   ├─→ Update approximate size
   └─→ Check ShouldFlush()
       └─→ If true, trigger flush
   ↓
4. Flush (when memtable full)
   ├─→ Create new memtable
   ├─→ Write old memtable to SSTable
   │   ├─→ Sort rows by EntityID
   │   ├─→ Write header
   │   ├─→ Write data section
   │   ├─→ Build index
   │   ├─→ Write index section
   │   └─→ Write footer
   └─→ UnifiedIndex.HotReload()
       └─→ Load new SSTable
```

### Write Performance

- **WAL Write**: ~1-2μs per entry (no fsync)
- **WAL Write (fsync)**: ~100-200μs per entry
- **Memtable Write**: <1μs (in-memory)
- **SSTable Flush**: ~10-50ms (256MB memtable)

---

## Read Path

### Step-by-Step Read Flow

```
1. Client Read Request
   ↓
2. StorageService.GetEntity(entityID)
   ├─→ Tenant Guard (Validate tenant)
   └─→ Read from:
       ├─→ Memtable (lock-free read)
       │   └─→ Copy-on-write snapshot
       └─→ SSTable Readers
           ├─→ Binary search in index
           ├─→ Seek to offset
           └─→ Read rows (mmap-friendly)
   ↓
3. Merge Results
   ├─→ Combine memtable + SSTable rows
   ├─→ Deduplicate by timestamp (newest wins)
   └─→ Deserialize to UnifiedEntity
```

### Read Performance

- **Memtable Read**: <1μs (cache hit)
- **SSTable Index Lookup**: ~10-50μs (binary search)
- **SSTable Row Read**: ~1-10μs per row (mmap)
- **Entity Deserialization**: ~10-100μs (depends on size)

---

## Compaction

### Size-Tiered Compaction Strategy

**Tiers**:
- Tier 0: Memtable (256MB)
- Tier 1: SSTables (~256MB each, 4 files)
- Tier 2: SSTables (~1GB each, 4 files)
- Tier 3: SSTables (~4GB each, 4 files)
- ...

**Compaction Trigger**:
- When tier has >= `size_tier_threshold` files (default: 4)
- Select files of similar size
- Merge into next tier

### Compaction Process

```
1. Planner.SelectSSTables()
   ├─→ Group by size tier
   ├─→ Select files to compact
   └─→ Return compaction job
   ↓
2. Executor.Execute(job)
   ├─→ Open all input SSTables
   ├─→ Create merge iterator
   ├─→ Stream merge:
   │   ├─→ Read rows in sorted order
   │   ├─→ Deduplicate (keep newest)
   │   └─→ Write to new SSTable
   ├─→ Write new SSTable
   └─→ Atomic swap:
       ├─→ Remove old SSTables
       └─→ Add new SSTable
   ↓
3. UnifiedIndex.HotReload()
   └─→ Reload SSTables
```

### Compaction Performance

- **Merge Rate**: ~100-500MB/s (depends on I/O)
- **Deduplication**: ~1M rows/sec
- **IO Throttling**: Configurable (default: 100MB/s, 1000 IOPS)

---

## Data Structures

### Memtable

```go
type Memtable struct {
    mu    sync.RWMutex
    data  map[EntityID][]Row
    size  int64  // Approximate size in bytes
    maxSize int64  // Flush threshold
}

type Row struct {
    Type     RowType
    EntityID EntityID
    Data     []byte
    Timestamp int64
}
```

**Memory Layout**:
- `data`: Map of entity ID to rows (append-only)
- `size`: Approximate memory usage (updated on Put)
- Copy-on-write for lock-free reads

### SSTable Reader

```go
type Reader struct {
    file     *os.File
    mmap     []byte  // Memory-mapped file
    header   *Header
    footer   *Footer
    index    []IndexEntry  // Loaded index
    indexMu  sync.RWMutex
}
```

**Memory Mapping**:
- Entire SSTable file mapped to memory
- Zero-copy reads
- OS handles page caching

### Compaction Manager

```go
type Manager struct {
    planner  *Planner
    executor *Executor
    jobs     chan CompactionJob
    workers  int  // Concurrent compactions
    throttle *IOThrottle
}
```

**IO Throttling**:
- Bandwidth limit: 100MB/s (configurable)
- IOPS limit: 1000 (configurable)
- Token bucket algorithm

---

## File Organization

### Directory Structure

```
/data/
├── wal/
│   ├── wal-00000001.wal  (128MB, active)
│   ├── wal-00000002.wal  (128MB, rotated)
│   └── wal-00000003.wal  (64MB, current)
├── sstable/
│   ├── sstable-00000001.sst  (256MB, tier 1)
│   ├── sstable-00000002.sst  (256MB, tier 1)
│   ├── sstable-00000003.sst  (1GB, tier 2)
│   └── sstable-00000004.sst  (1GB, tier 2)
└── handoff/
    └── hints/
        ├── node-1.hints
        └── node-2.hints
```

### File Lifecycle

1. **WAL Files**:
   - Created when WAL starts
   - Rotated when size limit reached
   - Deleted after successful flush (optional)

2. **SSTable Files**:
   - Created during memtable flush
   - Merged during compaction
   - Deleted after successful compaction

3. **Hint Files**:
   - Created when replica unavailable
   - Replayed when replica rejoins
   - Deleted after successful replay

---

## Error Handling

### Corruption Detection

- **WAL**: CRC32 per entry, stop on corruption
- **SSTable**: CRC32 per row, magic numbers in header/footer
- **Recovery**: WAL replay on startup

### Data Integrity

- **Checksums**: CRC32 on all data
- **Atomic Writes**: WAL + Memtable must both succeed
- **Atomic Compaction**: New SSTable written before old ones deleted

---

## Performance Tuning

### Write Performance

- **Disable WAL fsync**: Faster writes, risk of data loss
- **Increase memtable size**: Fewer flushes, more memory
- **Increase compaction concurrency**: Faster compaction, more I/O

### Read Performance

- **Increase cache sizes**: More hot data in memory
- **Use mmap**: Zero-copy reads
- **Reduce SSTable count**: Fewer files to check

### Space Efficiency

- **Lower size tier threshold**: More aggressive compaction
- **Enable compression**: (Future feature)
- **Tune compaction intervals**: Balance I/O vs. space

---

## Next Steps

- **API Usage**: See [API.md](./API.md)
- **Configuration**: See [CONFIGURATION.md](./CONFIGURATION.md)
- **Performance**: See [PERFORMANCE.md](./PERFORMANCE.md)

