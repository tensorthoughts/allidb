# ADR-004: No Global Index

## Status
Accepted

## Context
Traditional distributed databases often maintain global indexes that map keys to nodes. This provides:
- Single point of lookup
- Simplified routing
- Centralized metadata

However, global indexes have drawbacks:
- Single point of failure
- Scalability bottleneck
- Complex consistency requirements
- Network overhead for lookups

For a vector + graph database, we need efficient key-to-node routing without these limitations.

## Decision
We will **not use a global index**. Instead, we use:

### Consistent Hashing Ring
- **Virtual nodes**: Each physical node has multiple virtual nodes on the ring
- **SHA1 hashing**: Keys hash to positions on the ring
- **Replication factor**: Each key replicated to N nodes
- **Fast lookup**: O(log N) binary search for ownership
- **No coordination**: Each node can independently determine ownership

### Key Routing
- Client or any node can determine key ownership
- No need to query a central index
- Deterministic routing (same key → same nodes)
- Handles node additions/removals gracefully

### Partitioning
- Keys distributed across ring based on hash
- Virtual nodes provide load balancing
- Replication ensures redundancy
- No single point of failure

## Consequences

### Positive
- **No single point of failure**: No central index to fail
- **Scalability**: O(log N) lookup, no bottleneck
- **Decentralized**: Any node can route requests
- **Deterministic**: Same key always maps to same nodes
- **Elastic**: Easy to add/remove nodes
- **Low latency**: No network hop to index

### Negative
- **No global view**: Can't easily list all keys
- **Rebalancing**: Node changes require data movement
- **Hot spots**: Uneven key distribution possible
- **Complexity**: Consistent hashing logic

### Trade-offs
- Chose consistent hashing over global index for scalability
- Virtual nodes over simple hashing for load balancing
- Decentralized over centralized for availability
- Accept data movement cost for elasticity

## Implementation Notes
- Ring: `core/cluster/ring/ring.go`
- Coordinator uses ring for replica selection
- Gossip protocol for node membership (`core/cluster/gossip/`)
- Repair system handles rebalancing
- Virtual nodes configurable (default: 150 per node)
- SHA1 hashing for key distribution
- Binary search for O(log N) ownership lookup
- Replication factor configurable (default: 3)

## Alternatives Considered
1. **Global index**: Rejected due to scalability and availability concerns
2. **Static partitioning**: Rejected due to lack of elasticity
3. **Client-side routing**: Accepted - consistent hashing enables this

