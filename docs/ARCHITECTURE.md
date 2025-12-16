# AlliDB Architecture Documentation

## Table of Contents

1. [High-Level Architecture](#high-level-architecture)
2. [System Components](#system-components)
3. [Data Flow](#data-flow)
4. [Storage Architecture](#storage-architecture)
5. [Index Architecture](#index-architecture)
6. [Query Execution](#query-execution)
7. [Cluster Architecture](#cluster-architecture)
8. [Security Architecture](#security-architecture)

---

## High-Level Architecture

AlliDB is a distributed vector + graph database designed for RAG (Retrieval-Augmented Generation) workloads. It combines:

- **Vector Search**: Approximate Nearest Neighbor (ANN) search using HNSW
- **Graph Traversal**: Relationship-based candidate expansion
- **LSM-Tree Storage**: High-throughput write-optimized storage
- **Distributed System**: Consistent hashing, gossip protocol, quorum-based replication

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   gRPC API   │  │   REST API   │  │   HTTP API   │         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
└─────────┼──────────────────┼──────────────────┼─────────────────┘
          │                  │                  │
          └──────────────────┼──────────────────┘
                             │
          ┌──────────────────▼──────────────────┐
          │      Authentication & Authorization  │
          │  (API Key → Tenant ID, TLS/mTLS)    │
          └──────────────────┬──────────────────┘
                             │
          ┌──────────────────▼──────────────────┐
          │         Request Coordinator          │
          │  (Quorum reads/writes, routing)     │
          └──────────────────┬──────────────────┘
                             │
    ┌────────────────────────┼────────────────────────┐
    │                        │                        │
┌───▼────────┐      ┌────────▼────────┐      ┌───────▼──────┐
│   Storage  │      │  Query Executor │      │   Repair     │
│   Engine   │      │  + Unified Index│      │   System     │
└───┬────────┘      └─────────────────┘      └───────┬──────┘
    │                                                 │
┌───▼────────┐                              ┌────────▼────────┐
│    WAL     │                              │  Read Repair    │
│  Memtable  │                              │  Entropy Repair │
│  SSTable   │                              │  Hinted Handoff │
└───┬────────┘                              └─────────────────┘
    │
┌───▼──────────┐
│  Compaction  │
│   Manager    │
└──────────────┘
```

---

## System Components

### 1. Storage Layer

#### Write-Ahead Log (WAL)
- **Purpose**: Durability guarantee for writes
- **Location**: `core/storage/wal/`
- **Characteristics**:
  - Append-only binary format
  - Fsync before acknowledgment (configurable)
  - Single writer goroutine, multiple producers via channel
  - File rotation by size (default: 128MB)
  - CRC32 checksum per entry
  - Sequential writes for performance

**File Format**:
```
[Entry 1]
  [4 bytes: Length]
  [4 bytes: CRC32]
  [N bytes: Entry Data]
[Entry 2]
  ...
```

#### Memtable
- **Purpose**: In-memory write buffer
- **Location**: `core/storage/memtable/`
- **Characteristics**:
  - `map[EntityID][]Row` structure
  - Copy-on-write for lock-free reads
  - Approximate memory usage tracking
  - Flush trigger on size threshold (default: 256MB)
  - Append-only semantics

**Memory Layout**:
```
Memtable {
  data: map[EntityID][]Row
  size: int64 (approximate bytes)
  mu: RWMutex (for writes)
}
```

#### SSTable (Sorted String Table)
- **Purpose**: Immutable on-disk storage
- **Location**: `core/storage/sstable/`
- **File Format**:
  ```
  [Header]
    Magic: 4 bytes (0x414C4C49 = "ALLI")
    Version: 1 byte
    RowCount: 8 bytes
    Reserved: 3 bytes
  
  [Data Section]
    [Row 1]
      [4 bytes: Row Type]
      [4 bytes: Data Length]
      [N bytes: Row Data]
      [4 bytes: CRC32]
    [Row 2]
      ...
  
  [Index Section]
    [Entry 1]
      [EntityID: variable length]
      [Offset: 8 bytes]
    [Entry 2]
      ...
  
  [Footer]
    IndexOffset: 8 bytes
    IndexLength: 8 bytes
    Magic: 4 bytes (0x494C4C41 = "ILLA")
  ```

**Row Types**:
- `VECTOR` (0x01): Vector embedding row
- `EDGE` (0x02): Graph edge row
- `CHUNK` (0x03): Text chunk row
- `META` (0x04): Entity metadata row

### 2. Index Layer

#### Unified Index
- **Purpose**: Combines vector search and graph traversal
- **Location**: `core/index/unified/`
- **Components**:
  - HNSW index for ANN search
  - Graph cache for adjacency lists
  - Entity cache for hot entities
  - SSTable readers for cold data

**Query Flow**:
```
1. ANN Search (HNSW) → Top-K candidates
2. Graph Expansion → Neighbors of candidates
3. Unified Scoring → Combine vector + graph + importance
4. Top-K Selection → Final results
```

#### HNSW Index
- **Purpose**: Approximate Nearest Neighbor search
- **Location**: `core/index/hnsw/`
- **Algorithm**: Hierarchical Navigable Small World
- **Parameters**:
  - `M`: Maximum connections per layer (default: 16)
  - `EfConstruction`: Construction search width (default: 200)
  - `EfSearch`: Search width (default: 64)
  - `ShardCount`: Number of shards for concurrency (default: 16)

**Structure**:
```
Layer 2: [Entry Point] ──┐
                          │
Layer 1: [Node] ── [Node] ┼── [Node]
                          │
Layer 0: [Node] ── [Node] ┼── [Node] ── [Node]
```

### 3. Query Execution

#### Query Executor
- **Purpose**: Execute vector + graph queries
- **Location**: `core/query/`
- **Pipeline**:
  1. Embed query (if text provided)
  2. ANN vector search (HNSW)
  3. Graph expansion (traverse edges)
  4. Unified scoring (combine factors)
  5. Top-K selection

#### Unified Scorer
- **Purpose**: Combine multiple scoring factors
- **Formula**:
  ```
  score = w1 * cosine_similarity
        + w2 * graph_traversal_score
        + w3 * entity_importance
        + w4 * recency_decay
  ```
- **Default Weights**:
  - Cosine Similarity: 0.5
  - Graph Traversal: 0.3
  - Entity Importance: 0.15
  - Recency Decay: 0.05

### 4. Cluster Components

#### Consistent Hashing Ring
- **Purpose**: Key-to-node routing
- **Location**: `cluster/ring/`
- **Algorithm**: SHA1 hashing with virtual nodes
- **Replication**: Configurable replication factor (default: 3)

**Ring Structure**:
```
Hash Ring (0x0000 - 0xFFFF)
  ├─ Node1 (Virtual Nodes: 150)
  ├─ Node2 (Virtual Nodes: 150)
  └─ Node3 (Virtual Nodes: 150)
```

#### Gossip Protocol
- **Purpose**: Node discovery and state exchange
- **Location**: `cluster/gossip/`
- **Protocol**: Cassandra-style three-phase exchange
- **Phases**:
  1. SYN: Send local state
  2. ACK: Receive and merge remote state
  3. ACK2: Confirm merge

#### Coordinator
- **Purpose**: Request routing and quorum management
- **Location**: `cluster/coordinator.go`
- **Features**:
  - Quorum reads/writes
  - Replica selection
  - Response merging (majority voting)
  - Timeout handling

### 5. Repair System

#### Read Repair
- **Purpose**: Detect and fix replica divergence during reads
- **Location**: `cluster/repair/`
- **Flow**:
  1. Read from quorum replicas
  2. Compare responses
  3. Detect divergence
  4. Schedule repair asynchronously
  5. Execute repair via normal write path

#### Entropy Repair
- **Purpose**: Periodic full repair using Merkle trees
- **Location**: `cluster/repair/entropy/`
- **Process**:
  1. Build Merkle tree per partition
  2. Compare trees across replicas
  3. Identify divergent ranges
  4. Repair ranges using normal write path

#### Hinted Handoff
- **Purpose**: Handle writes when replicas are unavailable
- **Location**: `cluster/handoff/`
- **Flow**:
  1. Store write hints for unavailable nodes
  2. Replay hints when nodes rejoin
  3. Idempotent writes prevent duplicates

---

## Data Flow

### Write Path

```
Client Request
    ↓
gRPC/REST API
    ↓
Authentication (API Key → Tenant ID)
    ↓
Coordinator (Route to replicas)
    ↓
Storage Service
    ├─→ WAL (Append entry)
    ├─→ Memtable (In-memory buffer)
    └─→ Tenant Guard (Isolation check)
    ↓
Quorum Write (Replicate to N nodes)
    ↓
Response to Client
```

### Read Path

```
Client Query
    ↓
gRPC/REST API
    ↓
Authentication
    ↓
Query Executor
    ├─→ Unified Index
    │   ├─→ HNSW (ANN search)
    │   ├─→ Graph Cache (Traverse edges)
    │   └─→ Entity Cache (Hot entities)
    ├─→ SSTable Readers (Cold data)
    └─→ Unified Scorer (Combine scores)
    ↓
Coordinator (Quorum read if needed)
    ├─→ Read Repair (Detect divergence)
    └─→ Response Merging
    ↓
Top-K Results
    ↓
Response to Client
```

### Compaction Path

```
Compaction Manager (Periodic check)
    ↓
Planner (Select SSTables to compact)
    ├─→ Size-based selection
    └─→ Age-based selection
    ↓
Executor
    ├─→ Stream merge (Multiple SSTables)
    ├─→ Deduplicate (By timestamp)
    └─→ Write new SSTable
    ↓
Atomic Swap (Replace old SSTables)
    ↓
Unified Index (Hot reload new SSTables)
```

---

## Storage Architecture

### LSM-Tree Structure

```
Level 0: Memtable (In-memory, ~256MB)
    ↓ (Flush when full)
Level 1: SSTable-1, SSTable-2, ... (On-disk, ~256MB each)
    ↓ (Compact when threshold reached)
Level 2: SSTable-3, SSTable-4, ... (On-disk, ~1GB each)
    ↓ (Compact when threshold reached)
Level N: Larger SSTables
```

### File Organization

```
/data/
├── wal/
│   ├── wal-00000001.wal
│   ├── wal-00000002.wal
│   └── ...
├── sstable/
│   ├── sstable-00000001.sst
│   ├── sstable-00000002.sst
│   └── ...
└── handoff/
    └── hints/
        └── node-{id}.hints
```

### Data Persistence

1. **Write**: WAL → Memtable (both must succeed)
2. **Flush**: Memtable → SSTable (when size threshold)
3. **Compaction**: Multiple SSTables → Single SSTable
4. **Replication**: WAL + Memtable replicated to quorum

---

## Index Architecture

### Unified Index Structure

```
UnifiedIndex
├── HNSW Index (Vector search)
│   ├── Shard 0 (RWMutex)
│   ├── Shard 1 (RWMutex)
│   └── ... (16 shards)
├── Graph Cache (Adjacency lists)
│   └── LRU eviction (10,000 entities)
├── Entity Cache (Hot entities)
│   └── LRU eviction (1,000 entities)
└── SSTable Readers (Cold data)
    └── MergeReader (Multiple SSTables)
```

### Index Update Flow

```
SSTable Flush
    ↓
Unified Index (Hot Reload)
    ├─→ Scan new SSTable
    ├─→ Extract entities
    ├─→ Update HNSW (Add nodes)
    ├─→ Update Graph Cache (Add edges)
    └─→ Update Entity Cache (Add entities)
```

---

## Query Execution

### Query Pipeline

```
1. Query Vector (or embed text)
    ↓
2. ANN Search (HNSW)
    ├─→ Entry point (Top layer)
    ├─→ Navigate down layers
    └─→ Search in base layer (Ef candidates)
    ↓
3. Graph Expansion
    ├─→ For each candidate:
    │   ├─→ Get neighbors (Graph Cache)
    │   └─→ Add to candidate set
    └─→ Expand factor: 2x (default)
    ↓
4. Unified Scoring
    ├─→ Cosine similarity (vector)
    ├─→ Graph traversal score (edges)
    ├─→ Entity importance (metadata)
    └─→ Recency decay (timestamps)
    ↓
5. Top-K Selection
    └─→ Sort by score, return top K
```

### Performance Optimizations

- **Lock-free reads**: Copy-on-write in memtable
- **Sharded locks**: HNSW shards reduce contention
- **Caching**: Hot entities and graph edges cached
- **Batch operations**: BatchPutEntities for throughput
- **Async compaction**: Background I/O doesn't block writes

---

## Cluster Architecture

### Node Topology

```
                    ┌─────────┐
                    │  Node 1 │
                    └────┬────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
   ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
   │ Node 2  │◄────►│ Node 3  │◄────►│ Node 4  │
   └─────────┘      └─────────┘      └─────────┘
        │                │                │
        └────────────────┼────────────────┘
                         │
                    Gossip Protocol
                    (State Exchange)
```

### Replication Strategy

- **Replication Factor**: 3 (default, configurable)
- **Quorum Reads**: (RF/2) + 1 = 2 replicas
- **Quorum Writes**: (RF/2) + 1 = 2 replicas
- **Consistency Levels**:
  - `ONE`: Single replica
  - `QUORUM`: Majority of replicas
  - `ALL`: All replicas

### Failure Handling

1. **Node Failure Detection**: Gossip protocol (failure timeout: 5s)
2. **Hinted Handoff**: Store writes for unavailable nodes
3. **Read Repair**: Fix divergence during reads
4. **Entropy Repair**: Periodic full repair (default: 24 hours)

---

## Security Architecture

### Authentication

- **API Key**: `X-ALLIDB-API-KEY` header
- **Mapping**: API Key → Tenant ID
- **Comparison**: Constant-time comparison (crypto/subtle)

### Authorization

- **Tenant Isolation**: Enforced at storage layer
- **Tenant Guard**: Fails hard on violations
- **Context Injection**: Tenant ID in request context

### Transport Security

- **TLS**: Optional client TLS
- **mTLS**: Node-to-node mutual TLS
- **Cipher Suites**: Modern TLS 1.2+ only

---

## Performance Characteristics

### Write Performance

- **Throughput**: ~100K writes/sec (single node, no replication)
- **Latency**: <1ms (p99, memtable write)
- **Durability**: Fsync configurable (trade-off: latency vs. durability)

### Read Performance

- **ANN Search**: <10ms (p99, 1M vectors, k=10)
- **Graph Traversal**: <5ms (p99, 2-hop expansion)
- **Cache Hit Rate**: >80% (hot entities)

### Storage Efficiency

- **Compression**: None (raw binary format)
- **Space Overhead**: ~10% (indexes, metadata)
- **Compaction**: Reduces space by ~30% (removes duplicates)

---

## Scalability

### Horizontal Scaling

- **Sharding**: Consistent hashing ring
- **Replication**: Configurable replication factor
- **Load Balancing**: Client-side or proxy-based

### Vertical Scaling

- **Memory**: Memtable size configurable (default: 256MB)
- **CPU**: HNSW shards scale with cores
- **Disk**: SSTable compaction handles large datasets

---

## Next Steps

For detailed information on:
- **Storage Internals**: See [STORAGE.md](./STORAGE.md)
- **API Usage**: See [API.md](./API.md)
- **Configuration**: See [CONFIGURATION.md](./CONFIGURATION.md)
- **Performance Tuning**: See [PERFORMANCE.md](./PERFORMANCE.md)

