# ADR-007: Unified Vector + Graph Index

## Status
Accepted

## Context
AlliDB needs to support both vector similarity search and graph traversal in a unified way. Traditional approaches separate these concerns:
- Vector databases: Fast ANN search but no graph relationships
- Graph databases: Rich relationships but no vector search
- Separate indexes: Requires multiple queries and manual result merging

For RAG workloads, we need to:
- Find similar entities via vector search
- Expand candidates via graph relationships
- Score results combining both vector similarity and graph structure
- Cache hot entities for performance
- Reload indexes when data changes

## Decision
We will implement a **Unified Index** that combines:

### Components
1. **HNSW Index**:
   - Approximate Nearest Neighbor search
   - Hierarchical Navigable Small World algorithm
   - Configurable M, ef_construction, ef_search
   - Sharded for concurrent access
   - Lock-free reads, sharded locks for writes

2. **Graph Cache**:
   - In-memory adjacency lists
   - LRU eviction (configurable size)
   - Fast neighbor lookups
   - Multi-hop traversal support

3. **Entity Cache**:
   - Hot entity caching
   - LRU eviction (configurable size)
   - Reduces SSTable lookups

4. **SSTable Integration**:
   - MergeReader for multiple SSTables
   - Hot reload on SSTable updates
   - Background reload loop
   - Seamless integration with storage layer

### Query Flow
1. **ANN Search**: HNSW finds top-K similar vectors
2. **Graph Expansion**: Traverse edges from candidates
3. **Unified Scoring**: Combine vector similarity, graph score, importance, recency
4. **Top-K Selection**: Return best results

### Scoring Formula
```
score = w1 * cosine_similarity(vector, query)
      + w2 * graph_traversal_score(edges)
      + w3 * entity_importance
      + w4 * recency_decay(timestamp)
```

Default weights:
- Cosine Similarity: 0.5
- Graph Traversal: 0.3
- Entity Importance: 0.15
- Recency Decay: 0.05

## Consequences

### Positive
- **Unified Query**: Single query combines vector + graph
- **Better Results**: Graph expansion finds related entities
- **Performance**: Caching reduces I/O
- **Flexibility**: Configurable scoring weights
- **Hot Reload**: Automatic index updates
- **Scalability**: Sharded HNSW supports concurrent access

### Negative
- **Complexity**: More moving parts than single index
- **Memory Usage**: Multiple caches increase memory footprint
- **Tuning**: Multiple parameters to tune (HNSW, caches, scoring)
- **Consistency**: Hot reload may have brief inconsistency window

### Trade-offs
- Chose unified index over separate indexes for query simplicity
- HNSW over other ANN algorithms for quality/performance balance
- LRU caches over other eviction policies for simplicity
- Hot reload over full rebuild for availability
- Configurable weights over fixed formula for flexibility

## Implementation Notes
- Unified Index: `core/index/unified/unified_index.go`
- HNSW: `core/index/hnsw/hnsw.go`
- Graph Cache: `core/index/graph/cache.go`
- Query Executor: `core/query/executor.go`
- Scorer: `core/query/scorer.go`
- All components configurable via configuration system
- Hot reload interval configurable (default: 5 seconds)

## Query Performance
- ANN Search: <10ms (p99, 1M vectors)
- Graph Expansion: <5ms (p99, 2-hop)
- Cache Hit Rate: >80% (hot entities)
- Total Query Latency: <15ms (p99, k=10)

## Alternatives Considered
1. **Separate indexes**: Rejected - requires manual result merging
2. **Graph-first approach**: Rejected - slower for vector queries
3. **Fixed scoring**: Rejected - not flexible enough
4. **Full rebuild on updates**: Rejected - downtime unacceptable
5. **No caching**: Rejected - too slow for hot entities

