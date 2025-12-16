# Production Implementation Gaps

This document lists all comments in the codebase indicating incomplete or simplified production implementations.

**Last Updated**: After ring rebalancing, entity metadata decoding, query execution simplifications; full test pass (`go test ./...`)  
**Total Gaps**: 12 gaps (1 critical, 4 medium, 7 low)

## High Priority - Core Functionality

### 1. WAL Replay Entity Restoration ✅ **RESOLVED**
**File**: `cmd/allidbd/main.go`  
**Status**: ✅ Implemented  
**Change**: WAL replay now deserializes `EntityWALEntry` and restores entities into the memtable during startup (counts & restored bytes logged).  
**Remaining Risk**: None for basic restore; future enhancement could validate tenant guard context during replay.

### 2. GetEntity SSTable Lookup ✅ **RESOLVED**
**File**: `cmd/allidbd/storage_service.go`  
**Status**: ✅ Implemented  
**Change**: `GetEntity` now merges memtable and SSTables (newest-first scan). Tenant is validated and only matching-tenant entities are returned. Timestamp heuristics choose the newest row.  
**Remaining Risk**: SSTable timestamp heuristic uses leading 8 bytes; align with canonical row format in future.

### 3. Range Scan API ✅ **RESOLVED (remote-capable)**
**File**: `cmd/allidbd/main.go`, `storage_service.go`, `api/grpc/allidb.proto`, `api/grpc/server.go`  
**Status**: ✅ Implemented (local + remote via gRPC forwarding)  
**Change**: RangeScan RPC added with pagination and optional `node_id`; StorageService scans memtable+SSTables; server forwards to remote nodes using gossip-resolved addresses.  
**Remaining Risk**: Remote dial currently `grpc.WithInsecure`; enable TLS for production. Ensure gossip transports publish real addresses.

## Medium Priority - Production Readiness

### 4. API Key Management 🔴 **CRITICAL SECURITY ISSUE**
**File**: `cmd/allidbd/main.go`, `core/config/config.go`, `configs/allidb.yaml`  
**Status**: ⚠️ Partially improved (static keys now configurable), but still a security gap for production.  
**Change**: API keys are now loaded from `security.auth.static_api_keys` in config instead of being hardcoded in `main.go`.  
**Remaining Issue**: Static keys live in config files; there is no integration with a secret manager, rotation, or expiration.  
**Action**: 
- For production, load API keys from an external secret management system (Vault, AWS Secrets Manager, etc.) or identity provider.
- Avoid storing real production keys directly in YAML; use env/secret injection instead.
- Add rotation and optional expiration support.

### 5. Replica Client Hardening
**File**: `cmd/allidbd/main.go`, `api/grpc/allidb.proto`, `api/grpc/server.go`, `cmd/allidbd/storage_service.go`  
**Status**: ⚠️ Implemented but needs production hardening  
**Issue**: gRPC dial currently insecure; lacks pooling/retries/circuit breaker  
**Action**: Add TLS/auth, connection pooling, retries/backoff, circuit breaker

### 6. Gossip Transport Hardening
**File**: `cmd/allidbd/main.go`, `core/cluster/gossip/gossip.go`  
**Status**: ⚠️ Implemented TCP transport (JSON over TCP, optional TLS)  
**Issue**: Needs production-grade TLS/auth and verified address publication  
**Action**: Enforce TLS, authenticate peers, ensure nodes publish reachable addresses

### 7. Handoff Write Failure Tracking
**File**: `cmd/allidbd/main.go:504`
**Status**: ⚠️ Incomplete
**Issue**: Only stores hints for dead nodes, not for alive nodes that fail during write
```go
// In a more sophisticated implementation, we could also track which alive nodes failed
// and store hints for them, but for now we rely on the dispatcher to retry
```
**Impact**: If an alive node fails during write, hint is not stored and may be lost
**Action**: Track failed writes to alive nodes and store hints for them

### 8. Merkle Tree Serialization ✅ **RESOLVED**
**File**: `core/cluster/repair/entropy/merkle.go`
**Status**: ✅ Full tree serialization/deserialization implemented (JSON with structure, hashes, key ranges)
**Remaining Risk**: None for correctness; optional future binary encoding for efficiency

### 9. Partition Replica Detection ✅ **RESOLVED**
**File**: `core/cluster/repair/entropy/compare.go`
**Status**: ✅ Uses ring/coord to fetch replicas (no placeholder key)
**Remaining Risk**: None noted

### 10. Repair Executor Write Optimization ✅ **RESOLVED**
**File**: `core/cluster/repair/executor.go`
**Status**: ✅ Uses ReplicaClient to write only to divergent nodes (falls back to coordinator if nil)
**Remaining Risk**: Harden ReplicaClient with TLS/pooling/retries for production

## Low Priority - Optimizations

### 11. Gossip Random Selection
**File**: `core/cluster/gossip/gossip.go:237`
**Status**: ⚠️ Simplified
**Issue**: Uses sequential selection instead of random
```go
// In production, use proper random selection
```
**Impact**: May not distribute gossip evenly
**Action**: Implement random peer selection using crypto/rand

### 12. Gossip Three-Phase Protocol
**File**: `core/cluster/gossip/gossip.go:262`
**Status**: ⚠️ Simplified
**Issue**: Simplified gossip protocol, doesn't wait for ACK/ACK2
```go
// In a real implementation, we'd wait for ACK and send ACK2
// For now, this is a simplified version
```
**Impact**: Less reliable state synchronization
**Action**: Implement full three-phase gossip protocol (SYN/ACK/ACK2)

### 13. Ring Rebalancing ✅ **RESOLVED**
**File**: `core/cluster/ring/ring.go`
**Status**: ✅ Rebalance rebuilds/sorts virtual nodes from current node set to redistribute keys.
**Remaining Risk**: None noted.

### 14. Entity Metadata Decoding ✅ **RESOLVED**
**File**: `core/index/unified/unified_index.go`
**Status**: ✅ Meta rows now decode full `UnifiedEntity` (tenant, type, importance, timestamps); vectors/edges/chunks still applied newest-first. Fallback preserves entity ID if no meta row.
**Remaining Risk**: None noted.

### 15. Compaction Timestamp Extraction
**File**: `core/storage/compaction/executor.go:239, 262-263`
**Status**: ⚠️ Simplified
**Issue**: Simplified timestamp extraction heuristic
```go
// For now, we'll use a simple heuristic: prefer rows with more data (likely newer)
// This is a simplified version - in practice, you'd parse the actual row format.
// For now, we'll look for a timestamp in the first 8 bytes of data if available.
```
**Impact**: May not correctly identify newest row for deduplication
**Action**: Implement proper row format parsing with explicit timestamps

### 16. Metrics Key Generation
**File**: `core/observability/metrics.go:261`
**Status**: ⚠️ Simple Implementation
**Issue**: Uses simple string concatenation for metric keys
```go
// Simple key generation - in production, use a more efficient method
```
**Impact**: Performance may degrade with many metrics
**Action**: Optimize metric key generation (e.g., use string interning or sync.Pool)

### 17. LLM Schema Validation
**File**: `core/llm/extractor.go:175, 180-182`
**Status**: ⚠️ Placeholder
**Issue**: Basic validation instead of full schema validation
```go
// For now, we do basic checks
// This is a placeholder - full schema validation would require a library like
// For now, we rely on validateGraph for basic validation
```
**Impact**: May accept invalid entity/relation structures
**Action**: Implement full JSON schema validation using a library like github.com/xeipuuv/gojsonschema

### 18. Query Execution Simplifications ✅ **RESOLVED**
**File**: `core/query/executor.go`
**Status**: ✅ ANN uses MaxCandidates/ef caps; graph expansion uses bounded BFS up to MaxGraphHops with expandFactor limit and path tracking.
**Remaining Risk**: None noted.

## Security Considerations

### 19. API Key Store Methods
**File**: `core/security/auth/apikey.go:73`
**Status**: ⚠️ Security Note
**Issue**: Some methods should be restricted in production
```go
// Note: In production, this should be restricted or removed.
```
**Impact**: Potential security risk if exposed
**Action**: Review and restrict/remove as needed, add production mode checks

### 20. TLS Insecure Skip Verify
**File**: `core/security/tls/config.go:41, 137`
**Status**: ⚠️ Testing Only
**Issue**: Option exists for testing but should never be used in production
```go
// InsecureSkipVerify (for testing only - DO NOT use in production)
// Set insecure skip verify (for testing only)
```
**Impact**: Security risk if enabled in production
**Action**: Add validation to prevent use in production builds, add build tags

## Error Handling

### 21. Compaction Error Handling
**File**: `core/storage/compaction/manager.go:333`
**Status**: ⚠️ Basic
**Issue**: Basic error handling, may need retry logic
```go
// In production, you might want to retry or handle errors differently
```
**Impact**: Compaction failures may not be recovered
**Action**: Implement retry logic with exponential backoff and better error handling

## Summary

**Total Gaps Found**: 12
- **🔴 Critical (Must Fix)**: 1  
  - API Key Management (security)
- **🟡 Medium Priority**: 4  
  - Replica client hardening, gossip transport hardening, handoff failure tracking, compaction error handling
- **🟢 Low Priority**: 7  
  - Gossip random selection, gossip three-phase, compaction timestamp heuristic, metrics key generation, LLM schema validation, API key store methods, TLS insecure skip verify
- **Security**: 2 (API key methods, TLS insecure skip verify)
- **Error Handling**: 1 (Compaction retries)

**Current Test Status**: `go test ./...` ✅ (all packages pass; WAL replay restore + GetEntity SSTable lookup + range scan + replica/gossip transport enabled)

## Priority Breakdown

### 🔴 Critical Gaps (Block Production)
1. **API Key Management**: Hardcoded test key is a security risk

### 🟡 Medium Priority (Production Quality)
2-5. Replica client hardening; gossip transport hardening; handoff failure tracking; compaction error handling

### 🟢 Low Priority (Optimizations)
6-12. Gossip selection/three-phase; compaction timestamp heuristic; metrics key generation; LLM schema validation; API key store methods; TLS insecure skip verify

## Recommendations

### Before Production Deployment

1. **🔴 CRITICAL**: Fix API key management (security vulnerability)
2. **🟡 MEDIUM**: Harden replica client (TLS/pooling/circuit breaker)
3. **🟡 MEDIUM**: Harden gossip transport (TLS/auth; ensure address publication)
4. **🟡 MEDIUM**: Harden range scan (TLS, optional HTTP exposure)

### Post-Launch Optimizations

7. Address medium-priority gaps based on production metrics
8. Optimize low-priority items as needed
9. Add monitoring and alerting for identified gaps

## Implementation Status

✅ **Completed**:
- ✅ Configuration system fully integrated (100%)
- ✅ All hardcoded values moved to config
- ✅ Hinted handoff store and dispatcher (fully implemented)
- ✅ Handoff integration in main.go (wired up)
- ✅ Production partition/tree/data providers (implemented)
- ✅ Complete memtable flushing (implemented)
- ✅ Repair detector integration (implemented)
- ✅ Comprehensive documentation (Architecture, API, Usage, Storage, Performance, ADRs)
- ✅ Build system (Makefile) and Docker setup

⚠️ **In Progress / Needs Work**:
- 🔴 API key management
- 🟡 Hardening: range scan TLS, replica client TLS/pooling, gossip transport TLS/address publication
- 🟡 Various medium/low priority optimizations (see detailed list above)
