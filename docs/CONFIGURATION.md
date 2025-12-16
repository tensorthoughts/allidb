# AlliDB Configuration Reference

## Table of Contents

1. [Configuration File](#configuration-file)
2. [Environment Variables](#environment-variables)
3. [Configuration Sections](#configuration-sections)
4. [Validation](#validation)
5. [Examples](#examples)

---

## Configuration File

### Location

Default: `configs/allidb.yaml`

Can be specified via:
- Command line: `./allidbd -config /path/to/config.yaml`
- Environment variable: `ALLIDB_CONFIG=/path/to/config.yaml`

### Format

YAML format with nested sections.

---

## Configuration Sections

### Node Configuration

```yaml
node:
  id: "node-1"                    # Unique node identifier
  listen_address: "0.0.0.0"       # Listen address (0.0.0.0 = all interfaces)
  grpc_port: 7000                 # gRPC server port
  http_port: 7001                 # HTTP/REST server port
  data_dir: "/var/lib/allidb"     # Data directory
```

**Environment Variables**:
- `ALLIDB_NODE_ID`
- `ALLIDB_NODE_LISTEN_ADDRESS`
- `ALLIDB_NODE_GRPC_PORT`
- `ALLIDB_NODE_HTTP_PORT`
- `ALLIDB_NODE_DATA_DIR`

---

### Cluster Configuration

```yaml
cluster:
  seed_nodes:                     # Initial nodes for discovery
    - "node-1:7000"
    - "node-2:7000"
  replication_factor: 3           # Number of replicas (1-10)
  gossip_port: 7946               # Gossip protocol port
  gossip_interval_ms: 1000        # Gossip interval (min: 100ms)
  failure_timeout_ms: 5000        # Node failure timeout (min: 1000ms)
  cleanup_interval_ms: 60000      # Dead node cleanup interval (min: 1000ms)
  fanout: 3                       # Nodes to gossip with per round (1-10)
  virtual_nodes_per_node: 150     # Virtual nodes for consistent hashing (1-1000)
```

**Environment Variables**:
- `ALLIDB_CLUSTER_SEED_NODES` (comma-separated)
- `ALLIDB_CLUSTER_REPLICATION_FACTOR`
- `ALLIDB_CLUSTER_GOSSIP_PORT`
- `ALLIDB_CLUSTER_GOSSIP_INTERVAL_MS`
- `ALLIDB_CLUSTER_FAILURE_TIMEOUT_MS`
- `ALLIDB_CLUSTER_CLEANUP_INTERVAL_MS`
- `ALLIDB_CLUSTER_FANOUT`
- `ALLIDB_CLUSTER_VIRTUAL_NODES_PER_NODE`

---

### Storage Configuration

#### WAL (Write-Ahead Log)

```yaml
storage:
  wal:
    max_file_mb: 128              # Max WAL file size before rotation (MB)
    fsync: true                   # Fsync before acknowledging writes
```

**Environment Variables**:
- `ALLIDB_STORAGE_WAL_MAX_FILE_MB`
- `ALLIDB_STORAGE_WAL_FSYNC`

**Performance Impact**:
- `fsync: false`: Faster writes (~1-2μs), risk of data loss on crash
- `fsync: true`: Slower writes (~100-200μs), guaranteed durability

#### Memtable

```yaml
storage:
  memtable:
    max_mb: 256                   # Max memtable size before flush (MB)
```

**Environment Variables**:
- `ALLIDB_STORAGE_MEMTABLE_MAX_MB`

**Performance Impact**:
- Larger memtable: Fewer flushes, more memory usage
- Smaller memtable: More frequent flushes, less memory

#### SSTable

```yaml
storage:
  sstable:
    block_size_kb: 64             # SSTable block size (KB)
```

**Environment Variables**:
- `ALLIDB_STORAGE_SSTABLE_BLOCK_SIZE_KB`

#### Compaction

```yaml
storage:
  compaction:
    max_concurrent: 2             # Max concurrent compactions (1-10)
    size_tier_threshold: 4        # Files per tier before compaction
    min_size_mb: 1                # Min SSTable size to compact (MB)
    max_age_minutes: 60           # Max age before compaction (minutes)
    check_interval_ms: 30000      # Compaction check interval (ms)
    max_iops: 1000                # Max IOPS for compaction
    max_bandwidth_mbps: 100       # Max bandwidth for compaction (MB/s)
```

**Environment Variables**:
- `ALLIDB_STORAGE_COMPACTION_MAX_CONCURRENT`
- `ALLIDB_STORAGE_COMPACTION_SIZE_TIER_THRESHOLD`
- `ALLIDB_STORAGE_COMPACTION_MIN_SIZE_MB`
- `ALLIDB_STORAGE_COMPACTION_MAX_AGE_MINUTES`
- `ALLIDB_STORAGE_COMPACTION_CHECK_INTERVAL_MS`
- `ALLIDB_STORAGE_COMPACTION_MAX_IOPS`
- `ALLIDB_STORAGE_COMPACTION_MAX_BANDWIDTH_MBPS`

---

### Index Configuration

#### HNSW

```yaml
index:
  hnsw:
    m: 16                         # Max connections per layer (2-128)
    ef_construction: 200          # Construction search width (M-1000)
    ef_search: 64                 # Search width (1-1000)
    shard_count: 16               # Number of shards (1-64)
```

**Environment Variables**:
- `ALLIDB_INDEX_HNSW_M`
- `ALLIDB_INDEX_HNSW_EF_CONSTRUCTION`
- `ALLIDB_INDEX_HNSW_EF_SEARCH`
- `ALLIDB_INDEX_HNSW_SHARD_COUNT`

**Performance Impact**:
- Higher `M`: Better recall, more memory, slower inserts
- Higher `ef_construction`: Better quality, slower construction
- Higher `ef_search`: Better recall, slower queries
- More shards: Better concurrency, more overhead

#### Unified Index

```yaml
index:
  graph_cache_size: 10000         # Max entities in graph cache
  entity_cache_size: 1000         # Max entities in entity cache
  reload_interval_ms: 5000        # SSTable reload interval (ms)
```

**Environment Variables**:
- `ALLIDB_INDEX_GRAPH_CACHE_SIZE`
- `ALLIDB_INDEX_ENTITY_CACHE_SIZE`
- `ALLIDB_INDEX_RELOAD_INTERVAL_MS`

---

### Query Configuration

```yaml
query:
  default_consistency: "QUORUM"   # ONE, QUORUM, ALL
  max_graph_hops: 2               # Max graph traversal hops (0-10)
  timeout_ms: 500                 # Query timeout (ms, 1-60000)
  default_k: 10                   # Default number of results (1-10000)
  max_candidates: 1000            # Max candidates to consider (1-100000)
  scoring:
    cosine_similarity_weight: 0.5
    graph_traversal_weight: 0.3
    entity_importance_weight: 0.15
    recency_decay_weight: 0.05
```

**Environment Variables**:
- `ALLIDB_QUERY_DEFAULT_CONSISTENCY`
- `ALLIDB_QUERY_MAX_GRAPH_HOPS`
- `ALLIDB_QUERY_TIMEOUT_MS`
- `ALLIDB_QUERY_DEFAULT_K`
- `ALLIDB_QUERY_MAX_CANDIDATES`
- `ALLIDB_QUERY_SCORING_COSINE_SIMILARITY_WEIGHT`
- `ALLIDB_QUERY_SCORING_GRAPH_TRAVERSAL_WEIGHT`
- `ALLIDB_QUERY_SCORING_ENTITY_IMPORTANCE_WEIGHT`
- `ALLIDB_QUERY_SCORING_RECENCY_DECAY_WEIGHT`

---

### Security Configuration

#### Authentication

```yaml
security:
  auth:
    enabled: true                 # Enable API key authentication
    api_key_header: "X-ALLIDB-API-KEY"  # Header name
```

**Environment Variables**:
- `ALLIDB_SECURITY_AUTH_ENABLED`
- `ALLIDB_SECURITY_AUTH_API_KEY_HEADER`

#### TLS

```yaml
security:
  tls:
    enabled: false                # Enable TLS
    cert_file: "/path/to/cert.pem"  # Server certificate
    key_file: "/path/to/key.pem"    # Server private key
    ca_file: "/path/to/ca.pem"      # CA certificate (for mTLS)
```

**Environment Variables**:
- `ALLIDB_SECURITY_TLS_ENABLED`
- `ALLIDB_SECURITY_TLS_CERT_FILE`
- `ALLIDB_SECURITY_TLS_KEY_FILE`
- `ALLIDB_SECURITY_TLS_CA_FILE`

---

### Observability Configuration

```yaml
observability:
  log_level: "INFO"               # DEBUG, INFO, WARN, ERROR
  metrics_enabled: true           # Enable metrics collection
  tracing:
    enabled: false                # Enable distributed tracing
```

**Environment Variables**:
- `ALLIDB_OBSERVABILITY_LOG_LEVEL`
- `ALLIDB_OBSERVABILITY_METRICS_ENABLED`
- `ALLIDB_OBSERVABILITY_TRACING_ENABLED`

---

### Repair Configuration

#### Read Repair

```yaml
repair:
  read_repair_enabled: true       # Enable read-time repair
  read_repair:
    max_concurrent: 5             # Max concurrent repairs
    rate_limit_per_sec: 100       # Repair rate limit
```

**Environment Variables**:
- `ALLIDB_REPAIR_READ_REPAIR_ENABLED`
- `ALLIDB_REPAIR_READ_REPAIR_MAX_CONCURRENT`
- `ALLIDB_REPAIR_READ_REPAIR_RATE_LIMIT_PER_SEC`

#### Hinted Handoff

```yaml
repair:
  hinted_handoff:
    enabled: true                 # Enable hinted handoff
    max_hints_per_node: 10000     # Max hints per node
    max_size_mb: 1024             # Max hint store size (MB)
    ttl_minutes: 60               # Hint TTL (minutes)
    flush_interval_ms: 1000       # Flush interval (ms)
    replay_interval_ms: 5000      # Replay interval (ms)
```

**Environment Variables**:
- `ALLIDB_REPAIR_HINTED_HANDOFF_ENABLED`
- `ALLIDB_REPAIR_HINTED_HANDOFF_MAX_HINTS_PER_NODE`
- `ALLIDB_REPAIR_HINTED_HANDOFF_MAX_SIZE_MB`
- `ALLIDB_REPAIR_HINTED_HANDOFF_TTL_MINUTES`
- `ALLIDB_REPAIR_HINTED_HANDOFF_FLUSH_INTERVAL_MS`
- `ALLIDB_REPAIR_HINTED_HANDOFF_REPLAY_INTERVAL_MS`

#### Anti-Entropy

```yaml
repair:
  anti_entropy:
    enabled: true                 # Enable entropy repair
    interval_minutes: 1440        # Repair interval (minutes, 1-10080)
```

**Environment Variables**:
- `ALLIDB_REPAIR_ANTI_ENTROPY_ENABLED`
- `ALLIDB_REPAIR_ANTI_ENTROPY_INTERVAL_MINUTES`

---

### AI Configuration

#### Embeddings

```yaml
ai:
  embeddings:
    provider: "openai"            # openai, ollama, local, custom
    model: "text-embedding-3-large"
    dim: 3072                     # Embedding dimension (1-16384)
    timeout_ms: 3000              # Request timeout (ms, 100-60000)
```

**Environment Variables**:
- `ALLIDB_AI_EMBEDDINGS_PROVIDER`
- `ALLIDB_AI_EMBEDDINGS_MODEL`
- `ALLIDB_AI_EMBEDDINGS_DIM`
- `ALLIDB_AI_EMBEDDINGS_TIMEOUT_MS`

**Required Secrets**:
- `OPENAI_API_KEY` (for OpenAI provider)
- `AZURE_OPENAI_KEY` (for Azure OpenAI)
- `CUSTOM_AI_KEY` (for custom provider)

#### LLM

```yaml
ai:
  llm:
    provider: "openai"            # openai, ollama, local, custom
    model: "gpt-4o-mini"
    temperature: 0.0              # Temperature (0.0-2.0)
    max_tokens: 2048              # Max tokens
    timeout_ms: 5000              # Request timeout (ms)
```

**Environment Variables**:
- `ALLIDB_AI_LLM_PROVIDER`
- `ALLIDB_AI_LLM_MODEL`
- `ALLIDB_AI_LLM_TEMPERATURE`
- `ALLIDB_AI_LLM_MAX_TOKENS`
- `ALLIDB_AI_LLM_TIMEOUT_MS`

#### Entity Extraction

```yaml
ai:
  entity_extraction:
    enabled: true                 # Enable entity extraction
    max_retries: 2                # Max retries on failure
```

**Environment Variables**:
- `ALLIDB_AI_ENTITY_EXTRACTION_ENABLED`
- `ALLIDB_AI_ENTITY_EXTRACTION_MAX_RETRIES`

**Note**: Entity extraction requires LLM to be configured.

---

## Environment Variables

### Override Rules

1. **Prefix**: All environment variables must start with `ALLIDB_`
2. **Nested Fields**: Use underscore to separate nested fields
   - Example: `ALLIDB_STORAGE_WAL_MAX_FILE_MB`
3. **Override**: Environment variables override YAML values
4. **Type Support**: string, int, bool, float
5. **Unknown Keys**: Ignored (no error)
6. **Type Mismatch**: Error on type mismatch

### Examples

```bash
# Override node ID
export ALLIDB_NODE_ID=node-2

# Override replication factor
export ALLIDB_CLUSTER_REPLICATION_FACTOR=5

# Override WAL fsync
export ALLIDB_STORAGE_WAL_FSYNC=false

# Override log level
export ALLIDB_OBSERVABILITY_LOG_LEVEL=DEBUG
```

---

## Validation

Configuration is validated on startup. Errors include:

- **Node ID empty**: `node.id` must not be empty
- **Invalid ports**: Ports must be 1-65535
- **Invalid replication factor**: Must be 1-10
- **Invalid sizes**: WAL, memtable sizes must be > 0
- **Invalid HNSW parameters**: M >= 2, ef_construction >= M
- **Invalid AI config**: Provider must be supported, dim > 0
- **Entity extraction without LLM**: Requires LLM to be configured

---

## Examples

### Minimal Configuration

```yaml
node:
  id: "node-1"
  data_dir: "./data"

cluster:
  seed_nodes: ["node-1:7000"]
  replication_factor: 1

storage:
  wal:
    max_file_mb: 128
  memtable:
    max_mb: 256

index:
  hnsw:
    m: 16
    ef_construction: 200
    ef_search: 64

query:
  default_consistency: "ONE"
  timeout_ms: 5000

security:
  auth:
    enabled: false
```

### Production Configuration

```yaml
node:
  id: "node-1"
  listen_address: "0.0.0.0"
  grpc_port: 7000
  http_port: 7001
  data_dir: "/var/lib/allidb"

cluster:
  seed_nodes:
    - "node-1:7000"
    - "node-2:7000"
    - "node-3:7000"
  replication_factor: 3
  gossip_port: 7946
  gossip_interval_ms: 1000
  failure_timeout_ms: 5000

storage:
  wal:
    max_file_mb: 128
    fsync: true
  memtable:
    max_mb: 512
  compaction:
    max_concurrent: 4
    max_iops: 2000
    max_bandwidth_mbps: 200

index:
  hnsw:
    m: 32
    ef_construction: 400
    ef_search: 128
    shard_count: 32
  graph_cache_size: 50000
  entity_cache_size: 5000

query:
  default_consistency: "QUORUM"
  timeout_ms: 1000
  default_k: 20
  max_candidates: 2000

security:
  auth:
    enabled: true
  tls:
    enabled: true
    cert_file: "/etc/allidb/cert.pem"
    key_file: "/etc/allidb/key.pem"

observability:
  log_level: "INFO"
  metrics_enabled: true
  tracing:
    enabled: true

repair:
  read_repair_enabled: true
  hinted_handoff:
    enabled: true
    max_size_mb: 2048
  anti_entropy:
    enabled: true
    interval_minutes: 1440
```

### High-Performance Configuration

```yaml
node:
  id: "node-1"
  data_dir: "/fast-ssd/allidb"

storage:
  wal:
    max_file_mb: 256
    fsync: false  # Trade durability for speed
  memtable:
    max_mb: 1024  # Larger memtable
  compaction:
    max_concurrent: 8
    max_iops: 5000
    max_bandwidth_mbps: 500

index:
  hnsw:
    m: 64
    ef_construction: 800
    ef_search: 256
    shard_count: 64
  graph_cache_size: 100000
  entity_cache_size: 10000

query:
  timeout_ms: 2000
  max_candidates: 5000
```

---

## Next Steps

- **API Usage**: See [API.md](./API.md)
- **Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md)
- **Performance**: See [PERFORMANCE.md](./PERFORMANCE.md)

