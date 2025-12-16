# AlliDB: A Comprehensive Overview

## Table of Contents

1. [Introduction](#introduction)
2. [About Tensor Thoughts](#about-tensor-thoughts)
3. [Why AlliDB?](#why-allidb)
4. [The Name "Alli"](#the-name-alli)
5. [What Makes AlliDB Different](#what-makes-allidb-different)
6. [Key Features & Specialties](#key-features--specialties)
7. [Built-in LLM Integration](#built-in-llm-integration)
8. [Architecture Overview](#architecture-overview)
9. [Storage Architecture (Low-Level)](#storage-architecture-low-level)
10. [Data Structures](#data-structures)
11. [Component Interactions](#component-interactions)
12. [HTTP and gRPC APIs](#http-and-grpc-apis)
13. [Vector Search Implementation](#vector-search-implementation)
14. [Graph RAG Implementation](#graph-rag-implementation)
15. [Class Diagrams](#class-diagrams)
16. [Clustering & Distributed Protocols](#clustering--distributed-protocols)
17. [Configuration System](#configuration-system)
18. [Conclusion](#conclusion)

---

## Introduction

So, what's AlliDB? It's a **distributed vector + graph database** that's built specifically for **Retrieval-Augmented Generation (RAG)** workloads. The thing is, most databases treat vector search and graph traversal as completely separate things. AlliDB brings them together in one high-performance system that's designed to power the next generation of AI applications.

---

## About Tensor Thoughts

AlliDB is developed by **Tensor Thoughts**.

### Enterprise Edition & Hosted Services

Need enterprise features, managed hosting, or commercial support? We've got you covered. Just reach out:

- **Email**: contact@tensorthoughts.com
- **Website**: https://tensorthoughts.ai

Tensor Thoughts offers:
- **Enterprise Edition**: Advanced features, SLA guarantees, and priority support
- **Hosted Services**: Fully managed AlliDB clusters with automatic scaling and backups
- **Professional Services**: Custom integrations, migration assistance, and training
- **Support Plans**: 24/7 support, dedicated engineering resources, and custom development

---

## Why AlliDB?

### The Problem

Here's the thing: modern AI applications, especially RAG systems, have a real challenge. They need to combine:

1. **Vector Similarity Search**: Finding semantically similar content using embeddings
2. **Graph Relationships**: Understanding connections between entities (documents, concepts, people)
3. **High Write Throughput**: Ingesting large volumes of documents and embeddings
4. **Low-Latency Reads**: Fast query responses for real-time applications
5. **Distributed Scale**: Handling millions of entities across multiple nodes

Traditional solutions require:
- **Vector databases** (Pinecone, Weaviate) for similarity search
- **Graph databases** (Neo4j, ArangoDB) for relationships
- **Separate orchestration** to combine results
- **Manual result merging** and scoring

And honestly? That's a pain. You end up with:
- **Complexity**: Managing multiple systems is a nightmare
- **Latency**: Every network hop adds delay
- **Inconsistency**: Different data models and update patterns make things messy
- **Cost**: Paying for multiple infrastructure components adds up fast

### The Solution

So we built AlliDB to solve all of this. Here's what it gives you:

- **Unified Vector + Graph Index**: Single query interface for both vector similarity and graph traversal
- **LSM-Tree Storage**: Write-optimized architecture for high ingestion rates
- **Built-in LLM Integration**: Native support for embeddings and entity extraction
- **Distributed by Design**: Consistent hashing, gossip protocol, quorum-based replication
- **Production-Ready**: Tenant isolation, authentication, TLS, repair systems

---

## The Name "Alli"

AlliDB is named after **Alli**, my daughter. I wanted to build something that mattered, something that was both powerful and reliable—the kind of system you'd trust with important stuff. That's why it's got her name.

The name also works perfectly because it's all about **bringing together** (all-i) different capabilities—vectors, graphs, storage, and AI—into one unified system. No more juggling multiple databases!

---

## What Makes AlliDB Different

### 1. **Unified Vector + Graph Index**

Here's the cool part: while other databases treat vectors and graphs like they're from different planets, AlliDB brings them together:

- **Single Query Interface**: One query returns results combining vector similarity AND graph relationships
- **Unified Scoring**: Results scored by vector similarity, graph traversal, entity importance, and recency
- **Seamless Integration**: Graph expansion happens automatically during vector search

### 2. **LSM-Tree Architecture for Vectors**

Most vector databases stick with in-memory indexes or basic disk storage. We went with an LSM-Tree instead, and here's why it's awesome:

- **High Write Throughput**: 100K+ writes/sec per node
- **Durability**: Write-Ahead Log (WAL) ensures data safety
- **Efficient Compaction**: Automatic merging and deduplication
- **Scalable**: Handles billions of vectors across multiple nodes

### 3. **Built-in LLM Integration**

Here's another thing: AlliDB doesn't just store vectors—it actually generates them for you:

- **Embedding Generation**: Native support for OpenAI, Ollama, and custom providers
- **Entity Extraction**: Automatic extraction of entities and relationships from text
- **Schema Validation**: Ensures LLM outputs match expected formats
- **Retry Logic**: Handles malformed LLM responses gracefully

### 4. **Graph RAG Out of the Box**

Traditional RAG systems are pretty "flat"—they'll find similar documents but completely ignore relationships. AlliDB's Graph RAG is different:

- **Finds Similar Entities**: Vector search finds semantically similar content
- **Expands via Graph**: Traverses relationships to find related entities
- **Unified Scoring**: Combines vector similarity with graph structure
- **Multi-Hop Traversal**: Explores 2-3 hops from initial candidates

### 5. **Production-Ready from Day One**

Let's be honest: many vector databases are built for demos. AlliDB is built for real production use, which means it includes:

- **Multi-Tenancy**: Tenant isolation at the storage layer
- **Authentication**: API key-based auth with configurable headers
- **TLS/mTLS**: Encrypted node-to-node and client communication
- **Repair Systems**: Read repair, hinted handoff, anti-entropy
- **Observability**: Structured logging, Prometheus metrics, distributed tracing

---

## Key Features & Specialties

Alright, let's dive into what makes AlliDB tick. Here are the key features:

### 1. **High-Performance Write Path**

```
Client Request
    ↓
HTTP/gRPC API
    ↓
Tenant Guard (Isolation)
    ↓
WAL (Durable Log) ──┐
    ↓                │
Memtable (Memory)    │
    ↓                │
SSTable (Disk) ◄─────┘ (Replay on startup)
```

- **WAL First**: All writes go to WAL for durability
- **Memtable Buffer**: In-memory write buffer for speed
- **Async Flush**: Memtable flushes to SSTable in background
- **No Blocking**: Writes never block on disk I/O

### 2. **Efficient Read Path**

```
Query Request
    ↓
Unified Index
    ├─→ HNSW (Vector Search)
    ├─→ Graph Cache (Neighbor Lookup)
    └─→ Entity Cache (Hot Entities)
    ↓
Storage Layer
    ├─→ Memtable (Latest Data)
    └─→ SSTable (Historical Data)
    ↓
Merge & Return
```

- **Multi-Level Cache**: Entity cache → Graph cache → Memtable → SSTable
- **Lock-Free Reads**: Copy-on-write memtable enables concurrent reads
- **Merge Reader**: Combines results from multiple SSTables automatically

### 3. **Distributed Architecture**

- **Consistent Hashing**: Even key distribution across nodes
- **Virtual Nodes**: 150 virtual nodes per physical node for fine-grained distribution
- **Gossip Protocol**: Cassandra-style node discovery and failure detection
- **Quorum Replication**: Configurable consistency (ONE, QUORUM, ALL)

### 4. **Graph RAG Pipeline**

1. **Vector Search**: HNSW finds top-K similar entities
2. **Graph Expansion**: Traverse edges from candidates (1-3 hops)
3. **Unified Scoring**: Combine vector similarity + graph weights + importance + recency
4. **Top-K Selection**: Return best results

---

## Built-in LLM Integration

One of the things that makes AlliDB special is that it doesn't just store data—it can actually work with LLMs to generate embeddings and extract entities. Pretty neat, right?

### Embedding Generation

AlliDB supports multiple embedding providers, so you're not locked into one vendor:

```yaml
ai:
  embeddings:
    provider: "openai"  # or "ollama", "local", "custom"
    model: "text-embedding-3-large"
    dim: 3072
    timeout_ms: 3000
```

**Supported Providers**:
- **OpenAI**: Works with `text-embedding-3-large` and `text-embedding-3-small` (and probably whatever they release next)
- **Ollama**: Any Ollama embedding model works (like `nomic-embed-text`)
- **Local**: We're working on on-device model support (coming soon!)
- **Custom**: Got your own provider? No problem—there's a pluggable interface for that

### Entity Extraction

But wait, there's more! AlliDB can automatically extract entities and relationships from text, which is super handy:

```yaml
ai:
  llm:
    provider: "openai"
    model: "gpt-4o-mini"
    temperature: 0.0
  entity_extraction:
    enabled: true
    max_retries: 2
```

**Features**:
- **Strict JSON Output**: LLM responses validated against schema
- **Retry Logic**: Automatically retries on malformed responses
- **Relationship Extraction**: Discovers edges between entities
- **Schema Validation**: Ensures extracted data matches expected format

### Integration Points

1. **On Write**: Automatically generate embeddings for text fields
2. **On Query**: Embed query text before vector search
3. **On Extraction**: Extract entities and relationships from documents
4. **Pluggable**: Easy to add new providers via interfaces

---

## Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Layer                           │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│  │  gRPC API  │  │  REST API  │  │  HTTP API  │           │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘           │
└────────┼────────────────┼────────────────┼──────────────────┘
         │                │                │
         └────────────────┼────────────────┘
                          │
         ┌────────────────▼────────────────┐
         │   Authentication & Authorization │
         │  (API Key → Tenant, TLS/mTLS)   │
         └────────────────┬────────────────┘
                          │
         ┌────────────────▼────────────────┐
         │      Request Coordinator         │
         │  (Quorum reads/writes, routing) │
         └────────────────┬────────────────┘
                          │
    ┌─────────────────────┼─────────────────────┐
    │                     │                     │
┌───▼────────┐    ┌───────▼───────┐    ┌───────▼──────┐
│  Storage   │    │ Query Executor│    │    Repair    │
│  Engine    │    │  (Vector+Graph)│    │   System     │
└───┬────────┘    └───────┬───────┘    └───────┬──────┘
    │                     │                     │
    │  ┌──────────────────┼──────────────────┐  │
    │  │                  │                  │  │
┌───▼──▼──┐    ┌──────────▼──────────┐    ┌──▼──▼──┐
│   WAL   │    │   Unified Index      │    │ Gossip │
│ Memtable│    │  (HNSW + Graph)     │    │  Ring  │
│ SSTable │    │  (Entity Cache)      │    │        │
└─────────┘    └──────────────────────┘    └────────┘
```

### Component Responsibilities

1. **Client Layer**: HTTP/gRPC APIs for external access
2. **Authentication**: API key validation, tenant isolation
3. **Coordinator**: Routes requests, handles quorum logic
4. **Storage Engine**: WAL, Memtable, SSTable, Compaction
5. **Query Executor**: Vector search, graph expansion, unified scoring
6. **Repair System**: Read repair, hinted handoff, anti-entropy
7. **Cluster Layer**: Gossip, consistent hashing, replication

---

## Storage Architecture (Low-Level)

Alright, let's get into the weeds. Here's how AlliDB actually stores data under the hood.

### LSM-Tree Structure

AlliDB uses a **Log-Structured Merge-tree (LSM-Tree)** for high write throughput. Here's how it works:

```
┌─────────────────────────────────────────┐
│           Write Path                     │
├─────────────────────────────────────────┤
│  1. Write to WAL (durable log)          │
│  2. Write to Memtable (in-memory)       │
│  3. Flush Memtable → SSTable (async)    │
│  4. Compact SSTables (background)       │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│           Read Path                      │
├─────────────────────────────────────────┤
│  1. Check Memtable (latest data)        │
│  2. Check SSTables (historical data)    │
│  3. Merge results (newest wins)         │
└─────────────────────────────────────────┘
```

### WAL (Write-Ahead Log)

**What it does**: Makes sure your data is safe before we tell you the write succeeded. No data loss here!

**Format**:
```
[Header: 16 bytes]
  - Magic: 4 bytes (0xALLIDB)
  - Version: 4 bytes
  - Reserved: 8 bytes

[Entries: Variable]
  - Entry Length: 4 bytes (uint32)
  - Entry Type: 1 byte
  - Entry Data: Variable
  - CRC32: 4 bytes

[Footer: 8 bytes]
  - File Size: 8 bytes (int64)
```

**Features**:
- **Append-Only**: Never modified, only appended
- **CRC32 Checksums**: Detect corruption
- **File Rotation**: New file when max size reached
- **Replay on Startup**: Restore memtable from WAL

### Memtable Structure

**What it does**: It's an in-memory write buffer that makes writes super fast

**Data Structure**:
```go
type Memtable struct {
    data map[EntityID][]Row  // EntityID → List of rows (append-only)
    mu   sync.RWMutex        // Protects writes
    size int64               // Approximate memory usage
}

type Row interface {
    EntityID() EntityID
    Size() int
}
```

**Features**:
- **Append-Only Semantics**: New rows appended to entity's list
- **Copy-on-Write**: Snapshot for lock-free reads
- **Memory Tracking**: Approximate size for flush trigger
- **Concurrent Writes**: Lock-protected writes
- **Lock-Free Reads**: Snapshot-based reads

**Flush Trigger**: When `size >= max_mb`, memtable flushes to SSTable

### SSTable Structure

**What it does**: Stores your historical data on disk in an immutable format (once written, never changed)

**File Format**:
```
[Header: 32 bytes]
  - Magic: 4 bytes (0xSSTB)
  - Version: 4 bytes
  - Row Count: 8 bytes
  - Data Offset: 8 bytes
  - Index Offset: 8 bytes

[Data Section: Variable]
  - Rows (sorted by entity_id, then timestamp)
    - Row Type: 1 byte (VECTOR, EDGE, CHUNK, META)
    - Entity ID Length: 2 bytes
    - Entity ID: Variable
    - Timestamp: 8 bytes (nanoseconds)
    - Data Length: 4 bytes
    - Data: Variable
    - CRC32: 4 bytes

[Index Section: Variable]
  - Entity ID → Offset mapping (binary searchable)
    - Entity ID: Variable
    - Offset: 8 bytes (into data section)
    - Row Count: 4 bytes

[Footer: 16 bytes]
  - Index Offset: 8 bytes
  - Footer CRC32: 4 bytes
  - Magic: 4 bytes (0xFOOT)
```

**Row Types**:
- **META**: Full entity metadata (tenant, type, importance, timestamps)
- **VECTOR**: Vector embedding with version and timestamp
- **EDGE**: Graph edge (from, to, relation, weight)
- **CHUNK**: Text chunk with metadata

**Features**:
- **Immutable**: Never modified after creation
- **Sorted**: Rows sorted by entity_id, then timestamp (newest first)
- **Indexed**: Fast lookup by entity_id via binary search
- **Mmap-Friendly**: Can be memory-mapped for zero-copy reads
- **Mergeable**: Multiple SSTables merged by MergeReader

### Compaction

**What it does**: Merges multiple SSTables together, gets rid of duplicates, and saves you disk space. It's like spring cleaning for your database!

**Strategy**: Size-Tiered Compaction

```
Level 0: [SSTable-1] [SSTable-2] [SSTable-3]  (small, many)
    ↓
Level 1: [Merged-1] [Merged-2]                 (medium, fewer)
    ↓
Level 2: [Merged-Large]                        (large, few)
```

**Process**:
1. **Select SSTables**: Pick similar-sized SSTables
2. **Stream Merge**: Merge rows while reading (no full load)
3. **Deduplicate**: Keep newest row per entity_id
4. **Write New SSTable**: Write merged result
5. **Atomic Swap**: Replace old SSTables with new one

**Benefits**:
- **Space Efficiency**: Deduplication reduces storage by ~30%
- **Read Performance**: Fewer SSTables to check
- **Write Amplification**: Controlled via `max_concurrent`

---

## Data Structures

Let's talk about the core data structures. These are the building blocks of everything in AlliDB.

### UnifiedEntity

**What it is**: This is the core data structure that represents an entity in AlliDB. Think of it as the main container for everything about an entity.

```go
type UnifiedEntity struct {
    EntityID    EntityID
    TenantID    string
    EntityType  string
    
    // Versioned embeddings (multiple vectors per entity)
    Embeddings  []VectorEmbedding
    
    // Graph edges
    IncomingEdges  []GraphEdge
    OutgoingEdges  []GraphEdge
    
    // Text chunks
    Chunks      []TextChunk
    
    // Metadata
    Importance  float64
    CreatedAt   time.Time
    UpdatedAt   time.Time
    
    mu sync.RWMutex  // Thread-safe
}
```

**Features**:
- **Append-Only Updates**: New embeddings/edges/chunks appended
- **Versioned Embeddings**: Track embedding history
- **Thread-Safe**: RWMutex for concurrent access
- **Binary Serialization**: Efficient storage format
- **JSON Support**: Human-readable format

### GraphEdge

**What it is**: Represents a relationship between two entities. This is how we build the graph part of the database.

```go
type GraphEdge struct {
    FromEntity EntityID
    ToEntity   EntityID
    Relation   string      // e.g., "REFERENCES", "AUTHORED_BY"
    Weight     float64     // 0.0-1.0, strength of relationship
    Timestamp  time.Time
}
```

### VectorEmbedding

**What it is**: A versioned vector embedding. You can have multiple embeddings per entity, and we track which version is which.

```go
type VectorEmbedding struct {
    Vector    []float32
    Version   int
    Timestamp time.Time
}
```

### TextChunk

**What it is**: Text content that's associated with an entity. Useful for storing the actual text that was embedded.

```go
type TextChunk struct {
    ChunkID  string
    Text     string
    Metadata map[string]interface{}
}
```

---

## Component Interactions

### Write Flow

```
1. Client sends PutEntity request
   ↓
2. HTTP/gRPC API receives request
   ↓
3. Auth middleware validates API key → extracts tenant_id
   ↓
4. Request Coordinator routes to local node (or replicas)
   ↓
5. Storage Service:
   a. Tenant Guard validates tenant isolation
   b. Writes to WAL (durable)
   c. Writes to Memtable (fast)
   d. Checks if memtable should flush
   ↓
6. If flush needed:
   a. Memtable.GetAllData() → all entities
   b. SSTable Writer writes to disk
   c. Memtable.Reset()
   ↓
7. Unified Index:
   a. Adds entity to HNSW (vector index)
   b. Adds edges to Graph Cache
   c. Caches entity in Entity Cache
   ↓
8. Response sent to client
```

### Read Flow (Query)

```
1. Client sends Query request (text or vector)
   ↓
2. HTTP/gRPC API receives request
   ↓
3. Auth middleware validates API key
   ↓
4. Query Executor:
   a. If text: Embedder generates query vector
   b. Unified Index.Search():
      - HNSW finds top-K similar vectors
      - Graph Cache expands via edges
      - Entity Cache provides hot entities
   c. Unified Scoring:
      - Cosine similarity (vector)
      - Graph traversal score (edges)
      - Entity importance
      - Recency decay
   d. Top-K selection
   ↓
5. Storage Service (if entity details needed):
   a. Check Memtable (latest)
   b. Check SSTables (historical)
   c. Merge results
   ↓
6. Response sent to client
```

### Replication Flow

```
1. Coordinator determines replicas (via consistent hashing)
   ↓
2. For each replica:
   a. If local: Write directly
   b. If remote: Send ReplicaWrite gRPC call
   ↓
3. Remote node:
   a. Receives ReplicaWrite
   b. Writes to memtable (no WAL, assumes primary logged)
   c. Updates local index
   ↓
4. Coordinator waits for quorum
   ↓
5. Response sent to client
```

---

## HTTP and gRPC APIs

AlliDB exposes both HTTP REST and gRPC APIs, so you can use whatever fits your stack best.

### HTTP REST API

**Base URL**: `http://localhost:7001` (by default)

**Endpoints**:

1. **POST /entity** - Store a single entity
   ```bash
   curl -X POST http://localhost:7001/entity \
     -H "Content-Type: application/json" \
     -H "X-ALLIDB-API-KEY: test-api-key" \
     -d '{
       "entity": {
         "entity_id": "doc-1",
         "tenant_id": "tenant1",
         "embeddings": [{"vector": [0.1, 0.2, ...], "version": 1}]
       }
     }'
   ```

2. **POST /entities/batch** - Store multiple entities
3. **POST /query** - Vector + graph search
4. **GET /health** - Health check

**Authentication**: API key in `X-ALLIDB-API-KEY` header (configurable)

**Flow**:
```
HTTP Request
    ↓
HTTPAuthMiddleware (extracts API key → tenant_id)
    ↓
HTTP Handler (creates gRPC context with tenant)
    ↓
gRPC Client (calls local gRPC server)
    ↓
gRPC Server (processes request)
    ↓
HTTP Response
```

### gRPC API

**Port**: `7000` (by default)

**Service**: `AlliDB` (creative, I know)

**Methods**:

1. **PutEntity** - Store a single entity
2. **BatchPutEntities** - Store multiple entities
3. **Query** - Vector + graph search
4. **RangeScan** - Scan entities in a range
5. **ReplicaRead** - Internal replication read
6. **ReplicaWrite** - Internal replication write
7. **HealthCheck** - Health check

**Authentication**: API key in gRPC metadata (header: `x-allidb-api-key`)

**Flow**:
```
gRPC Request
    ↓
GRPCAuthInterceptor (extracts API key from metadata → tenant_id)
    ↓
gRPC Handler (processes request with tenant in context)
    ↓
gRPC Response
```

**Streaming Support**: Both unary and streaming RPCs supported

---

## Vector Search Implementation

So how does AlliDB actually find similar vectors? Let's break it down.

### HNSW Algorithm

AlliDB uses **Hierarchical Navigable Small World (HNSW)** for approximate nearest neighbor search. It's a pretty clever algorithm:

**Structure**:
```
Layer 2: [Entry Point] ──┐
         │                │
Layer 1: [Node] ──────────┼──→ [Node] ──→ [Node]
         │                │
Layer 0: [Node] ──────────┴──→ [Node] ──→ [Node] ──→ [Node]
         (All nodes, dense connections)
```

**Search Process**:
1. **Start at Entry Point** (top layer)
2. **Greedy Search**: Move to nearest neighbor
3. **Descend Layer**: When no closer neighbor found
4. **Repeat**: Until layer 0
5. **Refine**: Search with `ef_search` candidates

**Parameters**:
- **M**: Number of bidirectional links (default: 16)
- **ef_construction**: Candidate list size during build (default: 200)
- **ef_search**: Candidate list size during search (default: 64)

**Performance**:
- **Insert**: O(log N) where N = number of vectors
- **Search**: O(log N) for top-K results
- **Memory**: O(N * M) where M = average connections per node

### Integration with Storage

```
Query Vector
    ↓
HNSW.Search(query, k*expandFactor)
    ↓
Top-K Candidate Entity IDs
    ↓
Graph Cache.Expand(candidates, expandFactor)
    ↓
Expanded Candidate Entity IDs
    ↓
Unified Scoring
    ↓
Top-K Results
```

**Optimizations**:
- **Sharded HNSW**: 16 shards for concurrent access
- **Lock-Free Reads**: No locks during search
- **Sharded Locks**: Write locks per shard (not global)

---

## Graph RAG Implementation

This is where things get really interesting. Let's talk about Graph RAG.

### What is Graph RAG?

Traditional RAG is pretty straightforward:
1. Embed your query
2. Find similar documents (vector search)
3. Return the top-K documents

**The problem?** It completely ignores relationships between documents. That's a huge missed opportunity!

Graph RAG is smarter:
1. Embed your query
2. Find similar documents (vector search)
3. **Expand via graph edges** (find related documents)
4. **Score by vector + graph** (unified scoring)
5. Return top-K results

**Why this is better**: You get documents that are:
- Semantically similar (vector search finds them)
- **AND** related via graph (edges connect them)

This gives you way richer results!

### Implementation in AlliDB

#### Step 1: Vector Search (ANN)

```go
// Find initial candidates via HNSW
annCandidates := hnswIndex.Search(queryVector, k*expandFactor)
// Returns: ["entity-1", "entity-2", "entity-3", ...]
```

#### Step 2: Graph Expansion

```go
// Expand candidates via graph edges
expandedCandidates := make(map[EntityID]bool)

// Add initial candidates
for _, candidate := range annCandidates {
    expandedCandidates[candidate] = true
    
    // Get neighbors (1 hop)
    neighbors := graphCache.GetNeighbors(candidate)
    for _, neighbor := range neighbors {
        expandedCandidates[neighbor] = true
        
        // Get 2-hop neighbors (if expandFactor >= 2)
        if expandFactor >= 2 {
            neighbors2 := graphCache.GetNeighbors(neighbor)
            for _, n2 := range neighbors2 {
                expandedCandidates[n2] = true
            }
        }
    }
}
```

#### Step 3: Unified Scoring

```go
score = w1 * cosine_similarity(query, entity.vector)
      + w2 * graph_traversal_score(entity, path)
      + w3 * entity.importance
      + w4 * recency_decay(entity.updated_at)

// Default weights:
// w1 = 0.5 (vector similarity)
// w2 = 0.3 (graph traversal)
// w3 = 0.15 (importance)
// w4 = 0.05 (recency)
```

**Graph Traversal Score**:
- **Direct Connection**: High score if entity directly connected to query result
- **Path Weight**: Sum of edge weights along path
- **Hop Penalty**: Score decreases with distance (1-hop > 2-hop > 3-hop)

#### Step 4: Top-K Selection

```go
// Sort by score (descending)
sort.Slice(candidates, func(i, j int) bool {
    return candidates[i].score > candidates[j].score
})

// Return top K
return candidates[:k]
```

### Example: Document Knowledge Graph

```
Query: "What is machine learning?"

Step 1: Vector Search finds:
  - "Introduction to ML" (score: 0.95)
  - "ML Algorithms" (score: 0.88)
  - "Deep Learning Basics" (score: 0.82)

Step 2: Graph Expansion finds:
  - "Introduction to ML" → "ML Algorithms" (edge: REFERENCES)
  - "ML Algorithms" → "Neural Networks" (edge: USES)
  - "Deep Learning Basics" → "Neural Networks" (edge: EXPLAINS)

Step 3: Unified Scoring:
  - "Introduction to ML": 0.95 (vector) + 0.2 (graph) = 1.15
  - "ML Algorithms": 0.88 (vector) + 0.3 (graph, 2 connections) = 1.18
  - "Neural Networks": 0.65 (vector, lower) + 0.4 (graph, many connections) = 1.05

Step 4: Top-K Results:
  1. "ML Algorithms" (score: 1.18)
  2. "Introduction to ML" (score: 1.15)
  3. "Neural Networks" (score: 1.05)
```

**The key insight here**: "Neural Networks" might not make it into the top-K by vector similarity alone, but graph expansion finds it because it's highly connected to ML topics. That's the power of Graph RAG!

---

## Class Diagrams

### Storage Layer

```
┌─────────────────┐
│  StorageService │
├─────────────────┤
│ +PutEntity()    │
│ +GetEntity()    │
│ +RangeScan()    │
└────────┬────────┘
         │
    ┌────┴────┬──────────┬──────────┐
    │         │          │          │
┌───▼───┐ ┌──▼───┐  ┌────▼────┐ ┌───▼────┐
│  WAL  │ │Memtable│ │ SSTable │ │Tenant  │
│       │ │        │ │         │ │ Guard  │
└───────┘ └────────┘ └─────────┘ └────────┘
```

### Index Layer

```
┌──────────────────┐
│  UnifiedIndex    │
├──────────────────┤
│ +Search()        │
│ +AddEntity()     │
│ +GetEntity()     │
└────────┬─────────┘
         │
    ┌────┴────┬──────────┬──────────┐
    │         │          │          │
┌───▼───┐ ┌──▼────┐  ┌──▼──────┐ ┌─▼────────┐
│ HNSW  │ │ Graph │  │ Entity  │ │ SSTable  │
│ Index │ │ Cache │  │ Cache   │ │ Reader   │
└───────┘ └───────┘  └─────────┘ └──────────┘
```

### Query Layer

```
┌──────────────────┐
│  QueryExecutor   │
├──────────────────┤
│ +Query()         │
│ +QueryWithOpts() │
└────────┬─────────┘
         │
    ┌────┴────┬──────────┐
    │         │          │
┌───▼───┐ ┌──▼────┐  ┌──▼──────┐
│Unified│ │ Scorer│  │Embedder │
│ Index │ │       │  │         │
└───────┘ └───────┘  └─────────┘
```

### Cluster Layer

```
┌──────────────────┐
│   Coordinator    │
├──────────────────┤
│ +Write()         │
│ +Read()          │
│ +Query()         │
└────────┬─────────┘
         │
    ┌────┴────┬──────────┬──────────┐
    │         │          │          │
┌───▼───┐ ┌──▼────┐  ┌──▼──────┐ ┌─▼────────┐
│ Ring  │ │Gossip │  │ Repair  │ │ Handoff  │
│       │ │       │  │         │ │          │
└───────┘ └───────┘  └─────────┘ └──────────┘
```

### API Layer

```
┌──────────────────┐
│   gRPC Server    │
├──────────────────┤
│ +PutEntity()     │
│ +Query()         │
│ +RangeScan()     │
└────────┬─────────┘
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌──▼──────┐
│HTTP   │ │ gRPC    │
│Gateway│ │ Client  │
└───────┘ └─────────┘
```

---

## Clustering & Distributed Protocols

Alright, let's talk about how AlliDB works when you have multiple nodes. It uses a distributed architecture with consistent hashing, gossip-based membership, and quorum-based replication. This section gets into the nitty-gritty details—we're talking protocol-level stuff with byte-level encoding. If you're into that kind of thing, you'll love this section.

### Consistent Hashing Ring

AlliDB uses **consistent hashing** to distribute data across nodes. The cool part? Each physical node is represented by multiple **virtual nodes** on the ring. This gives you way better load distribution than just putting one node at one point on the ring.

#### Ring Structure

```
Physical Node: node-1
  ├─ Virtual Node 0: hash = SHA1("node-1:0")[0:8] → uint64
  ├─ Virtual Node 1: hash = SHA1("node-1:1")[0:8] → uint64
  ├─ ...
  └─ Virtual Node 149: hash = SHA1("node-1:149")[0:8] → uint64

Physical Node: node-2
  ├─ Virtual Node 0: hash = SHA1("node-2:0")[0:8] → uint64
  └─ ...
```

**Virtual Node Hash Calculation**:
```go
// Input: nodeID = "node-1", index = 0
key := fmt.Sprintf("%s:%d", nodeID, index)  // "node-1:0"
hash := SHA1(key)                            // 20 bytes
virtualNodeHash := uint64(hash[0])<<56 |     // Take first 8 bytes
                   uint64(hash[1])<<48 |
                   uint64(hash[2])<<40 |
                   uint64(hash[3])<<32 |
                   uint64(hash[4])<<24 |
                   uint64(hash[5])<<16 |
                   uint64(hash[6])<<8 |
                   uint64(hash[7])
```

**Key Hash Calculation**:
```go
// Input: key = "entity-123"
hash := SHA1(key)        // 20 bytes
keyHash := uint64(hash[0])<<56 | ...  // Same as virtual node
```

#### Ring Lookup Algorithm

1. **Hash the key** to get `keyHash` (uint64)
2. **Binary search** the sorted virtual nodes array for first `vnode.Hash >= keyHash`
3. **Wrap around** if at end of array (ring topology)
4. **Collect owners** by traversing clockwise, skipping duplicate physical nodes
5. **Return up to `replicationFactor`** unique physical nodes

**Time Complexity**: O(log V) where V = total virtual nodes (typically 150 * N physical nodes)

#### Ring Data Structure (In-Memory)

```go
type Ring struct {
    virtualNodes []VirtualNode  // Sorted by hash (ascending)
    nodes        map[NodeID]int // Physical node → virtual node count
    replicationFactor int       // Default: 3
    virtualNodesPerNode int     // Default: 150
}

type VirtualNode struct {
    Hash   uint64  // 8 bytes: position on ring
    NodeID NodeID  // string: physical node identifier
    Index  int     // 4 bytes: virtual node index (0-149)
}
```

**Memory Layout** (per virtual node):
- Hash: 8 bytes (uint64)
- NodeID: variable (string, typically 10-50 bytes)
- Index: 4 bytes (int32)
- **Total**: ~22-62 bytes per virtual node
- **For 3 nodes with 150 vnodes each**: ~10-18 KB

### Gossip Protocol

AlliDB uses a **Cassandra-style gossip protocol** for node membership and state synchronization. The protocol uses a **three-phase exchange** (SYN/ACK/ACK2) to efficiently propagate state.

#### Protocol Overview

```
Node A                                    Node B
  │                                         │
  │─── SYN (digest of known states) ───────>│
  │                                         │
  │<── ACK (digest + requested states) ────│
  │                                         │
  │─── ACK2 (any missing states) ──────────>│
  │                                         │
```

#### Message Types

**1. GossipDigestSyn (SYN)**

Purpose: Request state synchronization by sending a digest of known node states.

**In-Memory Structure**:
```go
type GossipDigestSyn struct {
    Digests []GossipDigest
}

type GossipDigest struct {
    NodeID    NodeID  // string: node identifier
    Generation int64  // 8 bytes: generation (incremented on restart)
    Heartbeat  int64  // 8 bytes: heartbeat version (monotonically increasing)
}
```

**Binary Encoding** (hypothetical wire format):
```
[Message Type: 1 byte] = 0x01 (SYN)
[Digest Count: 4 bytes] = uint32(len(Digests))
For each digest:
  [NodeID Length: 4 bytes] = uint32(len(NodeID))
  [NodeID: variable] = []byte(NodeID)
  [Generation: 8 bytes] = int64 (little-endian)
  [Heartbeat: 8 bytes] = int64 (little-endian)
```

**Example** (3 nodes):
```
0x01                    // SYN message
0x03 0x00 0x00 0x00     // 3 digests
  // Digest 1
  0x06 0x00 0x00 0x00   // NodeID length = 6
  "node-1"              // 6 bytes
  0x01 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Generation = 1
  0x05 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Heartbeat = 5
  // Digest 2
  0x06 0x00 0x00 0x00   // NodeID length = 6
  "node-2"              // 6 bytes
  0x01 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Generation = 1
  0x03 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Heartbeat = 3
  // Digest 3
  0x06 0x00 0x00 0x00   // NodeID length = 6
  "node-3"              // 6 bytes
  0x01 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Generation = 1
  0x07 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Heartbeat = 7
```

**Size**: ~1 + 4 + N*(4 + len(NodeID) + 8 + 8) bytes, where N = number of nodes

**2. GossipDigestAck (ACK)**

Purpose: Respond with digest and requested endpoint states.

**In-Memory Structure**:
```go
type GossipDigestAck struct {
    Digests []GossipDigest      // Our digest
    States  map[NodeID]*EndpointState  // Requested states
}

type EndpointState struct {
    NodeID          NodeID                    // string
    State           NodeState                 // 1 byte (Alive=0, Dead=1, Unknown=2)
    Heartbeat       int64                     // 8 bytes
    Generation      int64                     // 8 bytes
    LastUpdate      time.Time                 // 8 bytes (UnixNano)
    ApplicationState map[string]interface{}   // Variable (e.g., SSTable versions)
}
```

**Binary Encoding** (hypothetical):
```
[Message Type: 1 byte] = 0x02 (ACK)
[Digest Count: 4 bytes] = uint32(len(Digests))
[Digests: variable] = Same as SYN
[State Count: 4 bytes] = uint32(len(States))
For each state:
  [NodeID Length: 4 bytes]
  [NodeID: variable]
  [State: 1 byte] = NodeState (0=Alive, 1=Dead, 2=Unknown)
  [Generation: 8 bytes] = int64
  [Heartbeat: 8 bytes] = int64
  [LastUpdate: 8 bytes] = int64 (UnixNano)
  [AppState Count: 4 bytes] = uint32(len(ApplicationState))
  For each app state entry:
    [Key Length: 4 bytes]
    [Key: variable]
    [Value Type: 1 byte] = 0 (string), 1 (int64), 2 ([]byte)
    [Value Length: 4 bytes] (if string/bytes)
    [Value: variable]
```

**Example** (sending state for node-1):
```
0x02                    // ACK message
0x01 0x00 0x00 0x00     // 1 digest (our local state)
  // Digest (our state)
  0x06 0x00 0x00 0x00   // NodeID length = 6
  "node-1"              // 6 bytes
  0x01 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Generation = 1
  0x05 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Heartbeat = 5
0x01 0x00 0x00 0x00     // 1 state to send
  // State for node-1
  0x06 0x00 0x00 0x00   // NodeID length = 6
  "node-1"              // 6 bytes
  0x00                   // State = Alive
  0x01 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Generation = 1
  0x05 0x00 0x00 0x00 0x00 0x00 0x00 0x00  // Heartbeat = 5
  0xE8 0x03 0x00 0x00 0x00 0x00 0x00 0x00  // LastUpdate = 1000 (example)
  0x01 0x00 0x00 0x00   // 1 app state entry
    // App state: "sstable_version" = "v1.2.3"
    0x0F 0x00 0x00 0x00  // Key length = 15
    "sstable_version"    // 15 bytes
    0x00                 // Value type = string
    0x05 0x00 0x00 0x00  // Value length = 5
    "v1.2.3"             // 5 bytes
```

**3. GossipDigestAck2 (ACK2)**

Purpose: Final confirmation with any states the sender might need.

**In-Memory Structure**:
```go
type GossipDigestAck2 struct {
    States map[NodeID]*EndpointState  // States they might need
}
```

**Binary Encoding**: Same as ACK's state section (without digests).

#### Gossip Protocol Flow

**Phase 1: SYN**
1. Node A selects random peer (Node B) from known nodes
2. Node A builds digest of all known node states (NodeID, Generation, Heartbeat)
3. Node A sends SYN to Node B

**Phase 2: ACK**
1. Node B receives SYN
2. Node B compares digests:
   - If Node B has newer state → include in ACK
   - If Node A has newer state → request in ACK (by omitting from states)
3. Node B sends ACK with:
   - Its own digest
   - States Node A might need

**Phase 3: ACK2**
1. Node A receives ACK
2. Node A updates local state with received states
3. Node A compares digests and sends ACK2 with any states Node B might need
4. Node B receives ACK2 and updates local state

#### Failure Detection

Nodes are marked as **dead** if:
- `now - LastUpdate > FailureTimeout` (default: 60 seconds)

**State Transitions**:
- **Alive → Dead**: When `LastUpdate` exceeds timeout
- **Dead → Alive**: When receiving gossip with newer generation or heartbeat

#### Gossip Configuration

```yaml
cluster:
  gossip_interval_ms: 1000      # How often to gossip (1 second)
  failure_timeout_ms: 60000     # Mark node dead after 60 seconds
  fanout: 3                      # Number of peers to gossip with per round
  seed_nodes:                    # Initial nodes to contact
    - "node-1:7946"
    - "node-2:7946"
```

### Replication Protocol

AlliDB uses **quorum-based replication** for consistency. Writes must succeed on a quorum of replicas before being acknowledged.

#### Replica Selection

1. **Hash the key** using consistent hashing ring
2. **Get owners** from ring (up to `replicationFactor` nodes)
3. **Filter alive nodes** using gossip membership
4. **Select replicas** (up to `ReplicaCount`, default: 3)

#### Write Protocol

**Request Flow**:
```
Client Request
    ↓
Coordinator.Write(key, value)
    ↓
1. Ring.GetOwners(key) → [node-1, node-2, node-3]
2. Filter alive nodes → [node-1, node-2] (node-3 is dead)
3. If len(alive) < WriteQuorum → ERROR
4. Parallel writes:
   - ReplicaClient.Write(ctx, node-1, key, value)
   - ReplicaClient.Write(ctx, node-2, key, value)
5. Wait for WriteQuorum successes
6. Return success/error
```

**gRPC ReplicaWrite Message** (from `allidb.proto`):
```protobuf
message ReplicaWriteRequest {
  string key = 1;           // Entity ID
  string tenant_id = 2;     // Tenant ID
  bytes value = 3;          // Serialized UnifiedEntity (binary)
  string node_id = 4;       // Source node ID
}

message ReplicaWriteResponse {
  bool success = 1;
  string error = 2;
}
```

**Binary Encoding** (gRPC uses Protocol Buffers):
- Key: UTF-8 string (length-prefixed)
- TenantID: UTF-8 string (length-prefixed)
- Value: Raw bytes (UnifiedEntity.ToBytes())
- NodeID: UTF-8 string (length-prefixed)

**UnifiedEntity Binary Format** (see Data Structures section):
- EntityID: 4 bytes (length) + variable (UTF-8)
- TenantID: 4 bytes (length) + variable (UTF-8)
- EntityType: 4 bytes (length) + variable (UTF-8)
- Embeddings: 4 bytes (count) + N * (8 + 8 + 4 + M*4) bytes
- Edges: 4 bytes (count) + N * edge bytes
- Chunks: 4 bytes (count) + N * chunk bytes
- Importance: 8 bytes (float64, IEEE 754)
- CreatedAt: 8 bytes (int64, UnixNano)
- UpdatedAt: 8 bytes (int64, UnixNano)

#### Read Protocol

**Request Flow**:
```
Client Request
    ↓
Coordinator.Read(key)
    ↓
1. Ring.GetOwners(key) → [node-1, node-2, node-3]
2. Filter alive nodes → [node-1, node-2, node-3]
3. If len(alive) < ReadQuorum → ERROR
4. Parallel reads:
   - ReplicaClient.Read(ctx, node-1, key)
   - ReplicaClient.Read(ctx, node-2, key)
   - ReplicaClient.Read(ctx, node-3, key)
5. Wait for ReadQuorum successes
6. Merge responses (majority voting, newest timestamp wins)
7. Return merged value
```

**gRPC ReplicaRead Message**:
```protobuf
message ReplicaReadRequest {
  string key = 1;           // Entity ID
  string tenant_id = 2;     // Tenant ID
  string node_id = 3;       // Requesting node ID
}

message ReplicaReadResponse {
  bytes value = 1;          // Serialized UnifiedEntity (binary)
  bool success = 2;
  string error = 3;
}
```

**Response Merging**:
- **Majority Voting**: If 2/3 replicas return same value → use it
- **Timestamp Comparison**: If values differ, use newest (highest `UpdatedAt`)
- **Repair Trigger**: If values differ, schedule async repair

#### Quorum Configuration

```yaml
cluster:
  replication_factor: 3        # Total replicas per key
  read_quorum: 2               # Replicas needed for read (R)
  write_quorum: 2              # Replicas needed for write (W)
  # R + W > N ensures strong consistency (2 + 2 > 3)
```

**Consistency Levels**:
- **ONE**: Read/write from/to 1 replica (fast, eventual consistency)
- **QUORUM**: Read/write from/to (N/2 + 1) replicas (balanced)
- **ALL**: Read/write from/to all replicas (slow, strong consistency)

### Request Coordination

The **Coordinator** is responsible for routing requests to the correct replicas and handling quorum logic.

#### Coordinator Flow

```
┌─────────────────┐
│   Coordinator   │
├─────────────────┤
│                 │
│ 1. Get Ring     │───→ Consistent Hashing
│ 2. Get Gossip   │───→ Node Liveness
│ 3. Select Replicas│
│ 4. Send Requests│───→ ReplicaClient (gRPC)
│ 5. Wait Quorum  │
│ 6. Merge Results│
│ 7. Return       │
└─────────────────┘
```

#### Replica Client Interface

```go
type ReplicaClient interface {
    Read(ctx context.Context, nodeID NodeID, key string) ([]byte, error)
    Write(ctx context.Context, nodeID NodeID, key string, value []byte) error
}
```

**Implementation**: gRPC client that calls `ReplicaRead`/`ReplicaWrite` RPCs on remote nodes.

#### Timeout Handling

- **ReadTimeout**: Default 5 seconds
- **WriteTimeout**: Default 5 seconds
- **Context Cancellation**: All replica requests use `context.WithTimeout`
- **Early Termination**: If quorum reached, cancel remaining requests

#### Error Handling

- **Insufficient Replicas**: Return error if `len(alive) < quorum`
- **Quorum Not Reached**: Return error if successful responses < quorum
- **Partial Failures**: Continue with remaining replicas if some fail
- **Network Errors**: Retry with exponential backoff (future enhancement)

### Network Transport

**Current Implementation**: Uses gRPC over TCP for all node-to-node communication.

**Ports**:
- **gRPC**: 7000 (default) - Client API and replication
- **Gossip**: 7946 (default) - Membership and state sync

**Protocol Stack**:
```
Application Layer:  AlliDB Protocol (gRPC + Gossip)
Transport Layer:    gRPC (HTTP/2) or TCP (Gossip)
Network Layer:      TCP
Link Layer:         Ethernet/IP
```

**Future Enhancements**:
- UDP-based gossip for lower latency
- Compression for large entity transfers
- TLS/mTLS for encrypted node-to-node communication

---

## Configuration System

Let's talk about how you configure AlliDB. It's pretty straightforward, actually.

### Overview

AlliDB uses a **strongly-typed configuration system**, which means:
- **YAML files**: Human-readable configuration
- **Environment variables**: Override YAML values
- **Validation**: Strict validation on startup
- **Defaults**: Sensible defaults for all options

### Configuration Structure

```yaml
node:
  id: "node-1"
  listen_address: "0.0.0.0"
  grpc_port: 7000
  http_port: 7001
  data_dir: "/var/lib/allidb"

cluster:
  seed_nodes: ["node-1:7946", "node-2:7946"]
  replication_factor: 3
  gossip_port: 7946
  # ... more options

storage:
  wal:
    max_file_mb: 128
    fsync: true
  memtable:
    max_mb: 256
  # ... more options

# ... (see allidb.yaml.template for full structure)
```

### Environment Variable Overrides

All values can be overridden with `ALLIDB_` prefix:

```bash
export ALLIDB_NODE_ID=node-2
export ALLIDB_CLUSTER_REPLICATION_FACTOR=5
export ALLIDB_STORAGE_WAL_FSYNC=false
export ALLIDB_AI_EMBEDDINGS_PROVIDER=ollama
export ALLIDB_AI_EMBEDDINGS_DIM=768
```

**Naming Convention**: Nested fields use underscores:
- `cluster.replication_factor` → `ALLIDB_CLUSTER_REPLICATION_FACTOR`
- `storage.wal.fsync` → `ALLIDB_STORAGE_WAL_FSYNC`

### Validation

Configuration is validated on startup:
- **Required fields**: Node ID, ports, etc.
- **Value ranges**: Ports 1-65535, positive sizes, etc.
- **Dependencies**: Entity extraction requires LLM config
- **Type checking**: String, int, bool, float validation

### Configuration Files

1. **`allidb.yaml`**: Main configuration file
2. **`allidb.yaml.template`**: Comprehensive template with all options documented

See `configs/allidb.yaml.template` for complete documentation.

---

## Conclusion

So there you have it! AlliDB is a fresh take on database design for AI workloads. Here's what makes it special:

1. **Unified Architecture**: Vector search and graph traversal in one system (no more juggling multiple databases!)
2. **Production-Ready**: Multi-tenancy, authentication, TLS, repair systems—all the stuff you need for real deployments
3. **High Performance**: LSM-tree storage, lock-free reads, efficient compaction—it's fast
4. **Built-in AI**: Native LLM integration means you don't have to wire up embeddings yourself
5. **Graph RAG**: Automatic graph expansion gives you way richer search results
6. **Distributed**: Consistent hashing, gossip, quorum replication—it scales

Whether you're building a RAG system, a knowledge graph, or a semantic search engine, AlliDB gives you everything you need in one cohesive, well-designed system.

**Named with love for Alli, built with care for the future of AI applications.**

---

## Additional Resources

- **Architecture Details**: See `docs/ARCHITECTURE.md`
- **Storage Internals**: See `docs/STORAGE.md`
- **API Reference**: See `docs/API.md`
- **Usage Guide**: See `docs/USAGE.md`
- **Configuration**: See `configs/allidb.yaml.template`
- **ADRs**: See `docs/adr/` for design decisions

