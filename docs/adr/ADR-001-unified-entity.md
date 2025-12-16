# ADR-001: Unified Entity Model

## Status
Accepted

## Context
AlliDB needs to store and query entities that combine multiple data types:
- Vector embeddings for semantic search
- Graph edges for relationship traversal
- Text chunks for full-text content
- Metadata (importance scores, timestamps, tenant isolation)

Traditional databases typically separate these concerns into different tables or stores, requiring complex joins and multiple queries. For a vector + graph database optimized for RAG workloads, we need a unified model that can efficiently handle all these data types together.

## Decision
We will implement a **UnifiedEntity** model that combines:
- **EntityID**: Unique identifier for the entity
- **TenantID**: Multi-tenancy support
- **EntityType**: Type classification
- **Versioned Vector Embeddings**: Multiple embeddings with versioning
- **Incoming/Outgoing Graph Edges**: Bidirectional graph relationships
- **Text Chunks**: Associated text content with metadata
- **Importance Score**: Ranking/importance metric
- **Timestamps**: Created/updated tracking

The model supports:
- **Append-only updates**: Immutable history for auditability
- **Thread-safe reads**: Lock-free reads using copy-on-write
- **Binary + JSON serialization**: Efficient storage and API compatibility

## Consequences

### Positive
- **Single query access**: All entity data available in one structure
- **Efficient storage**: Co-located data reduces I/O
- **Simplified API**: One entity object instead of multiple queries
- **Versioning support**: Track embedding evolution over time
- **Graph-native**: Edges are first-class citizens
- **Multi-tenant ready**: Built-in tenant isolation

### Negative
- **Larger objects**: Entities can be large with many embeddings/chunks
- **Memory usage**: Full entity loaded even if only one field needed
- **Update complexity**: Append-only requires careful versioning

### Trade-offs
- Chose unified model over normalized schema for query performance
- Append-only over in-place updates for auditability and consistency
- Copy-on-write over fine-grained locking for read performance

## Implementation Notes
- Located in `core/entity/entity.go`
- Uses `sync.RWMutex` for thread safety
- Binary format optimized for SSTable storage
- JSON format for API compatibility
- Supports multiple vector embeddings with versioning
- Graph edges stored as separate `GraphEdge` structs (`core/entity/edge.go`)
- Text chunks stored as `TextChunk` structs with metadata (`core/entity/chunk.go`)
- Size estimation helpers for memory management
- Thread-safe read operations using RWMutex
- Append-only update methods (AddEmbedding, AddEdge, AddChunk)

