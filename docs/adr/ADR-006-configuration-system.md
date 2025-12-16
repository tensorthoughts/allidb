# ADR-006: Configuration System

## Status
Accepted

## Context
AlliDB has many configurable parameters across storage, indexing, clustering, security, and AI components. We need a configuration system that:
- Provides type safety
- Supports environment variable overrides
- Validates configuration on startup
- Has sensible defaults
- Is easy to extend
- Supports both YAML files and environment variables

Traditional approaches (flags, environment variables only, or config files only) don't provide the flexibility and type safety we need.

## Decision
We will implement a **strongly-typed configuration system** with:

### Configuration Structure
1. **Strongly Typed Structs**:
   - Nested structs matching YAML structure exactly
   - Type-safe access to all configuration values
   - Default values via `DefaultConfig()` function

2. **YAML File Support**:
   - Primary configuration via YAML file
   - Path configurable via CLI flag or environment variable
   - Fail-fast on malformed YAML

3. **Environment Variable Overrides**:
   - Prefix: `ALLIDB_` (required)
   - Nested fields separated by underscore
   - Example: `ALLIDB_STORAGE_WAL_MAX_FILE_MB`
   - Override YAML values
   - Support string, int, bool, float types
   - Ignore unknown keys (no error)
   - Error on type mismatch

4. **Validation**:
   - Strict validation on startup
   - Validates all required fields
   - Validates ranges and constraints
   - Clear error messages

5. **Secret Management**:
   - Separate secret provider abstraction
   - Load secrets from environment variables
   - Never log secrets
   - Fail-fast if required secret missing

### Configuration Sections
- **Node**: ID, addresses, ports, data directory
- **Cluster**: Seed nodes, replication factor, gossip settings
- **Storage**: WAL, memtable, SSTable, compaction settings
- **Index**: HNSW parameters, cache sizes, reload intervals
- **Query**: Consistency, timeouts, scoring weights
- **Security**: Authentication, TLS settings
- **Observability**: Logging, metrics, tracing
- **Repair**: Read repair, hinted handoff, anti-entropy
- **AI**: Embeddings, LLM, entity extraction

## Consequences

### Positive
- **Type Safety**: Compile-time checking of configuration access
- **Flexibility**: YAML for complex configs, env vars for overrides
- **Validation**: Catch configuration errors early
- **Documentation**: Struct fields serve as documentation
- **IDE Support**: Autocomplete and type checking
- **Default Values**: Sensible defaults reduce configuration burden
- **Secret Safety**: Secrets never logged or exposed

### Negative
- **Code Generation**: Need to maintain structs matching YAML
- **Complexity**: More code than simple key-value config
- **Reflection**: Environment variable parsing uses reflection
- **Maintenance**: Adding new config requires code changes

### Trade-offs
- Chose strongly-typed structs over map[string]interface{} for type safety
- YAML + env vars over flags for flexibility
- Fail-fast validation over runtime errors for reliability
- Separate secret provider over inline secrets for security

## Implementation Notes
- Configuration: `core/config/config.go` (structs and defaults)
- Loader: `core/config/loader.go` (YAML loading)
- Environment: `core/config/env.go` (env var overrides)
- Validation: `core/config/validate.go` (strict validation)
- Secrets: `core/security/secrets/provider.go` (secret management)
- All subsystems wired to use configuration structs
- Configuration loaded and validated in `main.go` before component initialization

## Configuration Flow
```
1. Load defaults (DefaultConfig())
2. Load YAML file (if provided)
3. Apply environment variable overrides
4. Validate configuration
5. Initialize components with config
```

## Examples

### YAML Configuration
```yaml
node:
  id: "node-1"
  grpc_port: 7000

storage:
  wal:
    max_file_mb: 128
    fsync: true
```

### Environment Variable Override
```bash
export ALLIDB_NODE_ID=node-2
export ALLIDB_STORAGE_WAL_FSYNC=false
```

### Validation Errors
- Node ID empty
- Invalid port ranges
- Replication factor out of range
- Missing required secrets
- Invalid AI provider names

## Alternatives Considered
1. **Flags only**: Rejected - too many flags, hard to manage
2. **Environment variables only**: Rejected - not user-friendly for complex configs
3. **YAML only**: Rejected - no easy way to override in containers
4. **Key-value store**: Rejected - no type safety, runtime errors
5. **Code generation from YAML**: Considered but rejected - adds build complexity

