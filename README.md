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

AlliDB is named after **Alli**, my daughter. I wanted to build something that mattered, something that was both powerful and reliable, the kind of system you'd trust with important stuff. That's why it's got her name.

The name also works perfectly because it's all about **bringing together** (all-i) different capabilities, vectors, graphs, storage, and AI, into one unified system. No more juggling multiple databases!

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

Here's another thing: AlliDB doesn't just store vectors, it actually generates them for you:

- **Embedding Generation**: Native support for OpenAI, Ollama, and custom providers
- **Entity Extraction**: Automatic extraction of entities and relationships from text
- **Schema Validation**: Ensures LLM outputs match expected formats
- **Retry Logic**: Handles malformed LLM responses gracefully

### 4. **Graph RAG Out of the Box**

Traditional RAG systems are pretty "flat", they'll find similar documents but completely ignore relationships. AlliDB's Graph RAG is different:

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

One of the things that makes AlliDB special is that it doesn't just store data, it can actually work with LLMs to generate embeddings and extract entities. Pretty neat, right?

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
- **Custom**: Got your own provider? No problem, there's a pluggable interface for that

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

For the full deep-dive (including protocol-level and byte-level details), see the original detailed doc at `docs/ALLIDB_OVERVIEW.md`.


