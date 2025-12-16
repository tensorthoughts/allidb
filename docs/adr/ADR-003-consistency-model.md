# ADR-003: Consistency Model

## Status
Accepted

## Context
AlliDB is a distributed vector + graph database that replicates data across multiple nodes. We need to define a consistency model that:
- Balances availability and consistency
- Supports RAG workloads (read-heavy with some writes)
- Handles network partitions gracefully
- Provides predictable behavior for users

Strong consistency (linearizability) provides the simplest model but limits availability. Eventual consistency provides high availability but complicates application logic. We need a middle ground.

## Decision
We will implement a **Quorum-based consistency model** with:

### Consistency Levels
1. **ONE**: Read/write from/to one replica (fastest, weakest consistency)
2. **QUORUM**: Read/write from/to majority of replicas (balanced)
3. **ALL**: Read/write from/to all replicas (strongest, slowest)

### Default Configuration
- **ReadQuorum**: 2 (out of 3 replicas)
- **WriteQuorum**: 2 (out of 3 replicas)
- **ReplicaCount**: 3

### Read Path
- Read from all alive replicas in parallel
- Wait for quorum of successful responses
- Merge responses (majority value wins)
- Return merged result

### Write Path
- Write to all alive replicas in parallel
- Wait for quorum of successful acknowledgments
- Return success if quorum reached

### Failure Handling
- Filter out dead nodes using gossip protocol
- Timeout handling with context cancellation
- Graceful degradation when quorum unavailable
- Hinted handoff for writes to unavailable nodes
- Read repair for automatic divergence detection and fixing
- Anti-entropy repair for periodic full consistency checks

## Consequences

### Positive
- **Configurable consistency**: Applications choose trade-offs per request
- **High availability**: Can operate with minority of nodes down
- **Predictable behavior**: Quorum ensures majority agreement
- **Network partition tolerance**: Can continue with majority partition
- **Read repair**: Detects and fixes divergence during reads (automatic)
- **Hinted handoff**: Writes succeed even when replicas temporarily unavailable
- **Anti-entropy repair**: Periodic full consistency checks using Merkle trees
- **Repair scheduling**: Rate-limited, asynchronous repair execution

### Negative
- **Stale reads possible**: With ONE consistency level
- **Write conflicts**: Last-write-wins may lose updates
- **Complexity**: Quorum logic adds coordination overhead
- **Latency**: QUORUM/ALL require waiting for multiple nodes

### Trade-offs
- Chose quorum over strong consistency for availability
- Majority voting over vector clocks for simplicity
- Last-write-wins over conflict resolution for performance
- Read repair over anti-entropy for proactive fixing

## Implementation Notes
- Coordinator: `core/cluster/coordinator.go`
- Gossip protocol: `core/cluster/gossip/` (Cassandra-style three-phase exchange)
- Repair system: `core/cluster/repair/`
  - Read repair: `detector.go`, `scheduler.go`, `executor.go`
  - Entropy repair: `entropy/merkle.go`, `entropy/compare.go`, `entropy/executor.go`, `entropy/orchestrator.go`
- Hinted handoff: `core/cluster/handoff/store.go`, `dispatcher.go`
- Configurable per-request via API (consistency_level field)
- Default consistency level configurable in query config
- Repair systems can be enabled/disabled via configuration

