# AlliDB Class Diagrams

## Table of Contents

1. [Storage Layer](#storage-layer)
2. [Index Layer](#index-layer)
3. [Query Layer](#query-layer)
4. [Cluster Layer](#cluster-layer)
5. [API Layer](#api-layer)

---

## Storage Layer

### WAL (Write-Ahead Log)

```
┌─────────────────────────────────────┐
│              WAL                    │
├─────────────────────────────────────┤
│ - config: Config                    │
│ - currentFile: *os.File             │
│ - currentSize: int64                │
│ - fileSequence: uint64              │
│ - writeCh: chan writeRequest        │
│ - closeCh: chan struct{}            │
│ - doneCh: chan struct{}             │
│ - wg: sync.WaitGroup                │
│ - mu: sync.RWMutex                  │
│ - closed: bool                      │
├─────────────────────────────────────┤
│ + New(config) (*WAL, error)         │
│ + Append(entry) error               │
│ + Replay(callback) error            │
│ + Close() error                     │
│ - writer()                          │
│ - writeEntry(entry) error           │
│ - rotateFile() error                │
└─────────────────────────────────────┘
```

### Memtable

```
┌─────────────────────────────────────┐
│           Memtable                  │
├─────────────────────────────────────┤
│ - mu: sync.RWMutex                  │
│ - data: map[EntityID][]Row          │
│ - size: int64                       │
│ - maxSize: int64                    │
├─────────────────────────────────────┤
│ + New(maxSize) *Memtable            │
│ + Put(row)                          │
│ + Get(entityID) []Row               │
│ + ShouldFlush() bool                │
│ + Reset()                           │
│ + GetAllEntities() []EntityID       │
│ + GetAllData() map[EntityID][]Row   │
└─────────────────────────────────────┘
```

### SSTable

```
┌─────────────────────────────────────┐
│          SSTable Writer             │
├─────────────────────────────────────┤
│ - file: *os.File                    │
│ - index: map[EntityID]int64         │
│ - currentOffset: int64              │
│ - rowCount: int64                   │
├─────────────────────────────────────┤
│ + NewWriter(path) (*Writer, error)  │
│ + WriteRow(row) error               │
│ + Close() error                     │
│ - writeHeader() error               │
│ - writeIndex() error                │
│ - writeFooter() error               │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│          SSTable Reader             │
├─────────────────────────────────────┤
│ - file: *os.File                    │
│ - mmap: []byte                      │
│ - header: *Header                   │
│ - footer: *Footer                   │
│ - index: []IndexEntry               │
│ - indexMu: sync.RWMutex             │
├─────────────────────────────────────┤
│ + NewReader(path) (*Reader, error)  │
│ + Get(entityID) ([]Row, error)      │
│ + Close() error                     │
│ - loadIndex() error                 │
│ - seekToEntity(entityID) (int64, error)│
└─────────────────────────────────────┘
```

### Compaction

```
┌─────────────────────────────────────┐
│      Compaction Manager             │
├─────────────────────────────────────┤
│ - planner: *Planner                 │
│ - executor: *Executor               │
│ - jobs: chan CompactionJob          │
│ - workers: int                      │
│ - throttle: *IOThrottle             │
│ - stopCh: chan struct{}             │
│ - wg: sync.WaitGroup                │
├─────────────────────────────────────┤
│ + NewManager(config) (*Manager, error)│
│ + Start() error                     │
│ + Stop()                            │
│ - compactionLoop()                  │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│        Compaction Planner           │
├─────────────────────────────────────┤
│ - config: PlannerConfig             │
├─────────────────────────────────────┤
│ + NewPlanner(config) *Planner       │
│ + SelectSSTables() ([]CompactionJob, error)│
│ - groupByTier() map[int][]SSTable   │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│       Compaction Executor           │
├─────────────────────────────────────┤
│ - config: ExecutorConfig            │
│ - throttle: *IOThrottle             │
├─────────────────────────────────────┤
│ + NewExecutor(config) *Executor     │
│ + Execute(job) error                │
│ - mergeSSTables(job) error          │
│ - deduplicateRows(rows) []Row       │
└─────────────────────────────────────┘
```

---

## Index Layer

### HNSW Index

```
┌─────────────────────────────────────┐
│              HNSW                   │
├─────────────────────────────────────┤
│ - config: Config                    │
│ - layers: [][]*Node                 │
│ - entryPoint: *Node                 │
│ - maxLayer: int                     │
│ - shards: []*HNSWShard              │
│ - shardCount: int                   │
│ - dimension: int                    │
│ - mu: []sync.RWMutex                │
├─────────────────────────────────────┤
│ + NewHNSW(config, dimension) *HNSW  │
│ + Add(node) error                   │
│ + Search(query, k) ([]SearchResult, error)│
│ - searchLayer(query, k, layer) []*Node│
│ - selectNeighbors(candidates, M) []*Node│
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│              Node                   │
├─────────────────────────────────────┤
│ - ID: uint64                        │
│ - Vector: []float32                 │
│ - Connections: [][]uint64           │
│ - Level: int                        │
└─────────────────────────────────────┘
```

### Unified Index

```
┌─────────────────────────────────────┐
│         UnifiedIndex                │
├─────────────────────────────────────┤
│ - hnswIndex: *hnsw.HNSW             │
│ - graphCache: *graph.GraphCache     │
│ - entityCache: *entityCache         │
│ - idMapping: map[EntityID]uint64    │
│ - reverseMapping: map[uint64]EntityID│
│ - sstableReaders: []*sstable.Reader │
│ - mergeReader: *sstable.MergeReader │
│ - reloadMu: sync.RWMutex            │
│ - lastReload: time.Time             │
│ - reloadInterval: time.Duration     │
│ - stopReload: chan struct{}         │
│ - reloadWg: sync.WaitGroup          │
│ - config: Config                    │
├─────────────────────────────────────┤
│ + New(config) (*UnifiedIndex, error)│
│ + Add(entity) error                 │
│ + Search(query, k) ([]SearchResult, error)│
│ + GetEntity(entityID) (*Entity, error)│
│ + HotReload() error                 │
│ + Close() error                     │
│ - reloadLoop()                      │
│ - loadSSTables() error              │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│          GraphCache                 │
├─────────────────────────────────────┤
│ - mu: sync.RWMutex                  │
│ - adjacency: map[EntityID][]Edge    │
│ - lru: *lru.Cache                   │
│ - maxSize: int                      │
├─────────────────────────────────────┤
│ + NewGraphCache(maxSize) *GraphCache│
│ + AddEdge(edge)                     │
│ + GetNeighbors(entityID) []EntityID │
│ + Traverse(start, hops) []EntityID  │
└─────────────────────────────────────┘
```

---

## Query Layer

### Query Executor

```
┌─────────────────────────────────────┐
│         Query Executor              │
├─────────────────────────────────────┤
│ - index: *unified.UnifiedIndex      │
│ - scorer: *Scorer                   │
│ - config: Config                    │
│ - traceEnabled: bool                │
│ - traceMu: sync.Mutex               │
│ - traces: []TraceEvent              │
├─────────────────────────────────────┤
│ + NewExecutor(index, config) *Executor│
│ + Query(ctx, queryVector, k) ([]QueryResult, error)│
│ + QueryWithOptions(ctx, queryVector, k, expandFactor) ([]QueryResult, error)│
│ + GetTrace() []TraceEvent           │
│ - trace(event, time, duration, data)│
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│            Scorer                   │
├─────────────────────────────────────┤
│ - weights: ScoringWeights           │
├─────────────────────────────────────┤
│ + NewScorer(weights) *Scorer        │
│ + Score(entity, queryVector, graphScore) float64│
│ - cosineSimilarity(a, b) float64    │
│ - recencyDecay(timestamp) float64   │
└─────────────────────────────────────┘
```

---

## Cluster Layer

### Coordinator

```
┌─────────────────────────────────────┐
│          Coordinator                │
├─────────────────────────────────────┤
│ - config: Config                    │
│ - ring: *ring.Ring                  │
│ - gossip: *gossip.Gossip            │
│ - replicaClient: ReplicaClient      │
│ - readQuorum: int                   │
│ - writeQuorum: int                  │
│ - readTimeout: time.Duration        │
│ - writeTimeout: time.Duration       │
├─────────────────────────────────────┤
│ + NewCoordinator(config, ring, gossip, client) *Coordinator│
│ + Write(ctx, key, value) error      │
│ + Read(ctx, key) ([]byte, error)    │
│ + ReadWithResponses(ctx, key) ([]Response, error)│
│ - selectReplicas(key) []NodeID      │
│ - mergeResponses(responses) []byte  │
└─────────────────────────────────────┘
```

### Ring (Consistent Hashing)

```
┌─────────────────────────────────────┐
│              Ring                   │
├─────────────────────────────────────┤
│ - config: Config                    │
│ - virtualNodes: []VirtualNode       │
│ - nodes: map[NodeID]*Node           │
│ - sorted: bool                      │
│ - mu: sync.RWMutex                  │
├─────────────────────────────────────┤
│ + New(config) *Ring                 │
│ + AddNode(nodeID)                   │
│ + RemoveNode(nodeID)                │
│ + GetOwners(key) []NodeID           │
│ + GetReplicas(key, rf) []NodeID     │
│ - hash(key) uint64                  │
│ - binarySearch(hash) int            │
└─────────────────────────────────────┘
```

### Gossip Protocol

```
┌─────────────────────────────────────┐
│            Gossip                   │
├─────────────────────────────────────┤
│ - config: Config                    │
│ - transport: Transport              │
│ - knownNodes: map[NodeID]*Node      │
│ - localState: *NodeState            │
│ - mu: sync.RWMutex                  │
│ - stopCh: chan struct{}             │
│ - wg: sync.WaitGroup                │
├─────────────────────────────────────┤
│ + NewGossip(config, transport) *Gossip│
│ + Start() error                     │
│ + Stop()                            │
│ + AddNode(node)                     │
│ + GetNodes() []*Node                │
│ - gossipLoop()                      │
│ - cleanupLoop()                     │
│ - exchangeState(node) error         │
└─────────────────────────────────────┘
```

### Repair System

```
┌─────────────────────────────────────┐
│        Repair Detector              │
├─────────────────────────────────────┤
│ - config: DetectorConfig            │
│ - scheduler: *Scheduler             │
│ - onDivergence: func(*Divergence)   │
│ - divergencesDetected: int64        │
│ - mu: sync.RWMutex                  │
├─────────────────────────────────────┤
│ + NewDetector(config) *Detector     │
│ + CheckReadResponses(ctx, key, responses) error│
│ - detectDivergence(responses) *Divergence│
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│       Repair Scheduler              │
├─────────────────────────────────────┤
│ - config: SchedulerConfig           │
│ - executor: *Executor               │
│ - queue: chan RepairTask            │
│ - rateLimiter: *rate.Limiter        │
│ - stopCh: chan struct{}             │
│ - wg: sync.WaitGroup                │
├─────────────────────────────────────┤
│ + NewScheduler(config) *Scheduler   │
│ + Start() error                     │
│ + Stop()                            │
│ + Schedule(task)                    │
│ - worker()                          │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│       Repair Executor               │
├─────────────────────────────────────┤
│ - coordinator: *Coordinator         │
├─────────────────────────────────────┤
│ + NewExecutor(config) *Executor     │
│ + Execute(task) error               │
└─────────────────────────────────────┘
```

---

## API Layer

### gRPC Server

```
┌─────────────────────────────────────┐
│         gRPC Server                 │
├─────────────────────────────────────┤
│ - server: *grpc.Server              │
│ - service: *AlliDBService           │
│ - auth: *auth.APIKeyAuth            │
│ - coordinator: CoordinatorService   │
│ - logger: *observability.Logger     │
├─────────────────────────────────────┤
│ + NewServer(config) *Server         │
│ + Start() error                     │
│ + Stop()                            │
│ - PutEntity(ctx, req) (*PutEntityResponse, error)│
│ - BatchPutEntities(ctx, req) (*BatchPutEntitiesResponse, error)│
│ - Query(ctx, req) (*QueryResponse, error)│
│ - HealthCheck(ctx, req) (*HealthCheckResponse, error)│
└─────────────────────────────────────┘
```

### HTTP Handlers

```
┌─────────────────────────────────────┐
│         HTTP Handlers               │
├─────────────────────────────────────┤
│ - grpcClient: AlliDBClient          │
│ - auth: *auth.APIKeyAuth            │
│ - logger: *observability.Logger     │
├─────────────────────────────────────┤
│ + NewHandlers(grpcClient, auth) *Handlers│
│ + PutEntity(w, r)                   │
│ + BatchPutEntities(w, r)            │
│ + Query(w, r)                       │
│ + HealthCheck(w, r)                 │
│ - handleError(w, err)               │
└─────────────────────────────────────┘
```

---

## Entity Layer

### UnifiedEntity

```
┌─────────────────────────────────────┐
│        UnifiedEntity                │
├─────────────────────────────────────┤
│ - mu: sync.RWMutex                  │
│ - EntityID: EntityID                │
│ - TenantID: string                  │
│ - EntityType: EntityType            │
│ - Embeddings: []VectorEmbedding     │
│ - IncomingEdges: []GraphEdge        │
│ - OutgoingEdges: []GraphEdge        │
│ - Chunks: []TextChunk               │
│ - Importance: float64               │
│ - CreatedAt: time.Time              │
│ - UpdatedAt: time.Time              │
├─────────────────────────────────────┤
│ + NewUnifiedEntity(id, tenant, type) *UnifiedEntity│
│ + AddEmbedding(vector)              │
│ + AddIncomingEdge(edge)             │
│ + AddOutgoingEdge(edge)             │
│ + AddChunk(chunk)                   │
│ + SetImportance(score)              │
│ + SerializeBinary() ([]byte, error) │
│ + DeserializeBinary(data) error     │
│ + SerializeJSON() ([]byte, error)   │
│ + DeserializeJSON(data) error       │
└─────────────────────────────────────┘
```

---

## Relationships

### Storage Layer Relationships

```
WAL ──┐
      ├──▶ StorageService ──▶ Memtable
      │                        │
      │                        ▼
      │                    SSTable (Writer)
      │                        │
      │                        ▼
      └──▶ Replay ─────────▶ SSTable (Reader)
                                │
                                ▼
                          UnifiedIndex
```

### Index Layer Relationships

```
UnifiedIndex
    ├──▶ HNSW (Vector Search)
    ├──▶ GraphCache (Graph Traversal)
    ├──▶ EntityCache (Hot Entities)
    └──▶ SSTable Readers (Cold Data)
            │
            ▼
        MergeReader
```

### Query Layer Relationships

```
QueryExecutor
    ├──▶ UnifiedIndex
    │       ├──▶ HNSW.Search()
    │       ├──▶ GraphCache.Traverse()
    │       └──▶ EntityCache.Get()
    └──▶ Scorer
            ├──▶ Cosine Similarity
            ├──▶ Graph Score
            ├──▶ Importance Score
            └──▶ Recency Decay
```

### Cluster Layer Relationships

```
Coordinator
    ├──▶ Ring (Routing)
    ├──▶ Gossip (Discovery)
    └──▶ ReplicaClient (Replication)
            │
            ▼
    RepairDetector
        ├──▶ RepairScheduler
        └──▶ RepairExecutor
```

---

## Next Steps

- **Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md)
- **API**: See [API.md](./API.md)
- **Configuration**: See [CONFIGURATION.md](./CONFIGURATION.md)

