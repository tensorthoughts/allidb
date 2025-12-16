package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate performs strict validation on the configuration.
// Returns an error if any validation fails.
func (c *Config) Validate() error {
	// Validate NodeConfig
	if err := c.validateNode(); err != nil {
		return fmt.Errorf("node config: %w", err)
	}

	// Validate ClusterConfig
	if err := c.validateCluster(); err != nil {
		return fmt.Errorf("cluster config: %w", err)
	}

	// Validate StorageConfig
	if err := c.validateStorage(); err != nil {
		return fmt.Errorf("storage config: %w", err)
	}

	// Validate IndexConfig
	if err := c.validateIndex(); err != nil {
		return fmt.Errorf("index config: %w", err)
	}

	// Validate QueryConfig
	if err := c.validateQuery(); err != nil {
		return fmt.Errorf("query config: %w", err)
	}

	// Validate SecurityConfig
	if err := c.validateSecurity(); err != nil {
		return fmt.Errorf("security config: %w", err)
	}

	// Validate ObservabilityConfig
	if err := c.validateObservability(); err != nil {
		return fmt.Errorf("observability config: %w", err)
	}

	// Validate RepairConfig
	if err := c.validateRepair(); err != nil {
		return fmt.Errorf("repair config: %w", err)
	}

	// Validate AIConfig
	if err := c.validateAI(); err != nil {
		return fmt.Errorf("ai config: %w", err)
	}

	return nil
}

// validateNode validates NodeConfig.
// Validates:
// - node.id not empty
// - ports in valid range (1-65535)
// - ports are different
// - data_dir exists or can be created
func (c *Config) validateNode() error {
	if c.Node.ID == "" {
		return fmt.Errorf("id is required")
	}

	if c.Node.ListenAddress == "" {
		return fmt.Errorf("listen_address is required")
	}

	if c.Node.GRPCPort <= 0 || c.Node.GRPCPort > 65535 {
		return fmt.Errorf("grpc_port must be between 1 and 65535, got %d", c.Node.GRPCPort)
	}

	if c.Node.HTTPPort <= 0 || c.Node.HTTPPort > 65535 {
		return fmt.Errorf("http_port must be between 1 and 65535, got %d", c.Node.HTTPPort)
	}

	if c.Node.GRPCPort == c.Node.HTTPPort {
		return fmt.Errorf("grpc_port and http_port must be different")
	}

	if c.Node.DataDir == "" {
		return fmt.Errorf("data_dir is required")
	}

	// Validate data directory exists or can be created
	absDataDir, err := filepath.Abs(c.Node.DataDir)
	if err != nil {
		return fmt.Errorf("invalid data_dir path: %w", err)
	}

	// Check if directory exists or can be created
	if info, err := os.Stat(absDataDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access data_dir: %w", err)
		}
		// Directory doesn't exist, try to create it
		if err := os.MkdirAll(absDataDir, 0755); err != nil {
			return fmt.Errorf("cannot create data_dir: %w", err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("data_dir is not a directory: %s", absDataDir)
	}

	return nil
}

// validateCluster validates ClusterConfig.
// Validates:
// - replication_factor > 0
// - seed nodes are valid
// - gossip and failure timeouts are sane
func (c *Config) validateCluster() error {
	if len(c.Cluster.SeedNodes) == 0 {
		return fmt.Errorf("at least one seed node is required")
	}

	for i, seed := range c.Cluster.SeedNodes {
		if seed == "" {
			return fmt.Errorf("seed_nodes[%d] cannot be empty", i)
		}
		// Basic validation: should contain ":" for host:port
		if len(seed) < 3 || !strings.Contains(seed, ":") {
			return fmt.Errorf("seed_nodes[%d] must be in format 'host:port', got '%s'", i, seed)
		}
	}

	if c.Cluster.ReplicationFactor < 1 {
		return fmt.Errorf("replication_factor must be at least 1, got %d", c.Cluster.ReplicationFactor)
	}

	if c.Cluster.ReplicationFactor > 10 {
		return fmt.Errorf("replication_factor should not exceed 10, got %d", c.Cluster.ReplicationFactor)
	}

	if c.Cluster.GossipIntervalMs < 100 {
		return fmt.Errorf("gossip_interval_ms must be at least 100ms, got %d", c.Cluster.GossipIntervalMs)
	}

	if c.Cluster.FailureTimeoutMs < 1000 {
		return fmt.Errorf("failure_timeout_ms must be at least 1000ms, got %d", c.Cluster.FailureTimeoutMs)
	}

	if c.Cluster.FailureTimeoutMs <= c.Cluster.GossipIntervalMs {
		return fmt.Errorf("failure_timeout_ms (%d) must be greater than gossip_interval_ms (%d)",
			c.Cluster.FailureTimeoutMs, c.Cluster.GossipIntervalMs)
	}

	if c.Cluster.GossipPort < 1 || c.Cluster.GossipPort > 65535 {
		return fmt.Errorf("gossip_port must be between 1 and 65535, got %d", c.Cluster.GossipPort)
	}

	if c.Cluster.CleanupIntervalMs < 1000 {
		return fmt.Errorf("cleanup_interval_ms must be at least 1000ms, got %d", c.Cluster.CleanupIntervalMs)
	}

	if c.Cluster.Fanout < 1 {
		return fmt.Errorf("fanout must be at least 1, got %d", c.Cluster.Fanout)
	}

	if c.Cluster.Fanout > 10 {
		return fmt.Errorf("fanout should not exceed 10, got %d", c.Cluster.Fanout)
	}

	if c.Cluster.VirtualNodesPerNode < 1 {
		return fmt.Errorf("virtual_nodes_per_node must be at least 1, got %d", c.Cluster.VirtualNodesPerNode)
	}

	if c.Cluster.VirtualNodesPerNode > 1000 {
		return fmt.Errorf("virtual_nodes_per_node should not exceed 1000, got %d", c.Cluster.VirtualNodesPerNode)
	}

	return nil
}

// validateStorage validates StorageConfig.
// Validates:
// - WAL + memtable sizes > 0
// - SSTable block size > 0
// - Compaction parameters are sane
func (c *Config) validateStorage() error {
	// Validate WAL
	if c.Storage.WAL.MaxFileMB < 1 {
		return fmt.Errorf("wal.max_file_mb must be at least 1, got %d", c.Storage.WAL.MaxFileMB)
	}

	if c.Storage.WAL.MaxFileMB > 10240 { // 10GB max
		return fmt.Errorf("wal.max_file_mb should not exceed 10240 (10GB), got %d", c.Storage.WAL.MaxFileMB)
	}

	// Validate Memtable
	if c.Storage.Memtable.MaxMB < 1 {
		return fmt.Errorf("memtable.max_mb must be at least 1, got %d", c.Storage.Memtable.MaxMB)
	}

	if c.Storage.Memtable.MaxMB > 10240 { // 10GB max
		return fmt.Errorf("memtable.max_mb should not exceed 10240 (10GB), got %d", c.Storage.Memtable.MaxMB)
	}

	// Validate SSTable
	if c.Storage.SSTable.BlockSizeKB < 1 {
		return fmt.Errorf("sstable.block_size_kb must be at least 1, got %d", c.Storage.SSTable.BlockSizeKB)
	}

	if c.Storage.SSTable.BlockSizeKB > 1024 { // 1MB max
		return fmt.Errorf("sstable.block_size_kb should not exceed 1024 (1MB), got %d", c.Storage.SSTable.BlockSizeKB)
	}

	// Validate Compaction
	if c.Storage.Compaction.MaxConcurrent < 1 {
		return fmt.Errorf("compaction.max_concurrent must be at least 1, got %d", c.Storage.Compaction.MaxConcurrent)
	}

	if c.Storage.Compaction.MaxConcurrent > 10 {
		return fmt.Errorf("compaction.max_concurrent should not exceed 10, got %d", c.Storage.Compaction.MaxConcurrent)
	}

	if c.Storage.Compaction.SizeTierThreshold < 2 {
		return fmt.Errorf("compaction.size_tier_threshold must be at least 2, got %d", c.Storage.Compaction.SizeTierThreshold)
	}

	return nil
}

// validateIndex validates IndexConfig.
// Validates:
// - HNSW parameters are sane (M >= 2, ef_construction >= M, ef_search >= 1)
func (c *Config) validateIndex() error {
	// Validate HNSW
	if c.Index.HNSW.M < 2 {
		return fmt.Errorf("index.hnsw.m must be at least 2, got %d", c.Index.HNSW.M)
	}

	if c.Index.HNSW.M > 128 {
		return fmt.Errorf("index.hnsw.m should not exceed 128, got %d", c.Index.HNSW.M)
	}

	if c.Index.HNSW.EfConstruction < c.Index.HNSW.M {
		return fmt.Errorf("index.hnsw.ef_construction (%d) must be at least m (%d)",
			c.Index.HNSW.EfConstruction, c.Index.HNSW.M)
	}

	if c.Index.HNSW.EfConstruction > 1000 {
		return fmt.Errorf("index.hnsw.ef_construction should not exceed 1000, got %d", c.Index.HNSW.EfConstruction)
	}

	if c.Index.HNSW.EfSearch < 1 {
		return fmt.Errorf("index.hnsw.ef_search must be at least 1, got %d", c.Index.HNSW.EfSearch)
	}

	if c.Index.HNSW.EfSearch > 1000 {
		return fmt.Errorf("index.hnsw.ef_search should not exceed 1000, got %d", c.Index.HNSW.EfSearch)
	}

	if c.Index.HNSW.ShardCount < 1 {
		return fmt.Errorf("index.hnsw.shard_count must be at least 1, got %d", c.Index.HNSW.ShardCount)
	}

	if c.Index.HNSW.ShardCount > 64 {
		return fmt.Errorf("index.hnsw.shard_count should not exceed 64, got %d", c.Index.HNSW.ShardCount)
	}

	if c.Index.GraphCacheSize < 0 {
		return fmt.Errorf("index.graph_cache_size must be non-negative, got %d", c.Index.GraphCacheSize)
	}

	if c.Index.EntityCacheSize < 0 {
		return fmt.Errorf("index.entity_cache_size must be non-negative, got %d", c.Index.EntityCacheSize)
	}

	if c.Index.ReloadIntervalMs < 100 {
		return fmt.Errorf("index.reload_interval_ms must be at least 100ms, got %d", c.Index.ReloadIntervalMs)
	}

	if c.Index.ReloadIntervalMs > 600000 { // 10 minutes max
		return fmt.Errorf("index.reload_interval_ms should not exceed 600000 (10 minutes), got %d", c.Index.ReloadIntervalMs)
	}

	return nil
}

// validateQuery validates QueryConfig.
func (c *Config) validateQuery() error {
	validConsistency := map[string]bool{
		"ONE":    true,
		"QUORUM": true,
		"ALL":    true,
	}

	if !validConsistency[c.Query.DefaultConsistency] {
		return fmt.Errorf("query.default_consistency must be ONE, QUORUM, or ALL, got '%s'",
			c.Query.DefaultConsistency)
	}

	if c.Query.MaxGraphHops < 0 {
		return fmt.Errorf("query.max_graph_hops must be non-negative, got %d", c.Query.MaxGraphHops)
	}

	if c.Query.MaxGraphHops > 10 {
		return fmt.Errorf("query.max_graph_hops should not exceed 10, got %d", c.Query.MaxGraphHops)
	}

	if c.Query.TimeoutMs < 1 {
		return fmt.Errorf("query.timeout_ms must be at least 1, got %d", c.Query.TimeoutMs)
	}

	if c.Query.TimeoutMs > 60000 { // 60 seconds max
		return fmt.Errorf("query.timeout_ms should not exceed 60000 (60s), got %d", c.Query.TimeoutMs)
	}

	if c.Query.DefaultK < 1 {
		return fmt.Errorf("query.default_k must be at least 1, got %d", c.Query.DefaultK)
	}

	if c.Query.DefaultK > 10000 {
		return fmt.Errorf("query.default_k should not exceed 10000, got %d", c.Query.DefaultK)
	}

	if c.Query.MaxCandidates < 1 {
		return fmt.Errorf("query.max_candidates must be at least 1, got %d", c.Query.MaxCandidates)
	}

	if c.Query.MaxCandidates > 100000 {
		return fmt.Errorf("query.max_candidates should not exceed 100000, got %d", c.Query.MaxCandidates)
	}

	return nil
}

// validateSecurity validates SecurityConfig.
func (c *Config) validateSecurity() error {
	// Validate Auth
	if c.Security.Auth.APIKeyHeader == "" {
		return fmt.Errorf("security.auth.api_key_header is required when auth is enabled")
	}

	for i, k := range c.Security.Auth.StaticAPIKeys {
		if k.Key == "" {
			return fmt.Errorf("security.auth.static_api_keys[%d].key is required", i)
		}
		if k.TenantID == "" {
			return fmt.Errorf("security.auth.static_api_keys[%d].tenant_id is required", i)
		}
	}

	// Validate TLS
	if c.Security.TLS.Enabled {
		if c.Security.TLS.CertFile == "" {
			return fmt.Errorf("security.tls.cert_file is required when TLS is enabled")
		}

		if c.Security.TLS.KeyFile == "" {
			return fmt.Errorf("security.tls.key_file is required when TLS is enabled")
		}

		// Validate cert file exists
		if _, err := os.Stat(c.Security.TLS.CertFile); err != nil {
			return fmt.Errorf("security.tls.cert_file does not exist: %w", err)
		}

		// Validate key file exists
		if _, err := os.Stat(c.Security.TLS.KeyFile); err != nil {
			return fmt.Errorf("security.tls.key_file does not exist: %w", err)
		}

		// If CA file is specified, validate it exists
		if c.Security.TLS.CAFile != "" {
			if _, err := os.Stat(c.Security.TLS.CAFile); err != nil {
				return fmt.Errorf("security.tls.ca_file does not exist: %w", err)
			}
		}
	}

	return nil
}

// validateObservability validates ObservabilityConfig.
func (c *Config) validateObservability() error {
	validLogLevels := map[string]bool{
		"DEBUG": true,
		"INFO":  true,
		"WARN":  true,
		"ERROR": true,
	}

	if !validLogLevels[c.Observability.LogLevel] {
		return fmt.Errorf("observability.log_level must be DEBUG, INFO, WARN, or ERROR, got '%s'",
			c.Observability.LogLevel)
	}

	return nil
}

// validateRepair validates RepairConfig.
func (c *Config) validateRepair() error {
	// Validate HintedHandoff
	if c.Repair.HintedHandoff.Enabled {
		if c.Repair.HintedHandoff.MaxHintsPerNode < 1 {
			return fmt.Errorf("repair.hinted_handoff.max_hints_per_node must be at least 1, got %d",
				c.Repair.HintedHandoff.MaxHintsPerNode)
		}

		if c.Repair.HintedHandoff.TTLMinutes < 1 {
			return fmt.Errorf("repair.hinted_handoff.ttl_minutes must be at least 1, got %d",
				c.Repair.HintedHandoff.TTLMinutes)
		}

		if c.Repair.HintedHandoff.TTLMinutes > 10080 { // 7 days max
			return fmt.Errorf("repair.hinted_handoff.ttl_minutes should not exceed 10080 (7 days), got %d",
				c.Repair.HintedHandoff.TTLMinutes)
		}

		if c.Repair.HintedHandoff.MaxSizeMB < 1 {
			return fmt.Errorf("repair.hinted_handoff.max_size_mb must be at least 1, got %d",
				c.Repair.HintedHandoff.MaxSizeMB)
		}

		if c.Repair.HintedHandoff.MaxSizeMB > 1048576 { // 1TB max
			return fmt.Errorf("repair.hinted_handoff.max_size_mb should not exceed 1048576 (1TB), got %d",
				c.Repair.HintedHandoff.MaxSizeMB)
		}

		if c.Repair.HintedHandoff.FlushIntervalMs < 100 {
			return fmt.Errorf("repair.hinted_handoff.flush_interval_ms must be at least 100ms, got %d",
				c.Repair.HintedHandoff.FlushIntervalMs)
		}

		if c.Repair.HintedHandoff.FlushIntervalMs > 60000 { // 60 seconds max
			return fmt.Errorf("repair.hinted_handoff.flush_interval_ms should not exceed 60000 (60s), got %d",
				c.Repair.HintedHandoff.FlushIntervalMs)
		}
	}

	// Validate AntiEntropy
	if c.Repair.AntiEntropy.Enabled {
		if c.Repair.AntiEntropy.IntervalMinutes < 1 {
			return fmt.Errorf("repair.anti_entropy.interval_minutes must be at least 1, got %d",
				c.Repair.AntiEntropy.IntervalMinutes)
		}

		if c.Repair.AntiEntropy.IntervalMinutes > 10080 { // 7 days max
			return fmt.Errorf("repair.anti_entropy.interval_minutes should not exceed 10080 (7 days), got %d",
				c.Repair.AntiEntropy.IntervalMinutes)
		}
	}

	return nil
}

// validateAI validates AIConfig.
// Validates:
// - AI embedding dim > 0
// - AI provider names supported (openai, ollama, local, custom)
// - entity extraction only enabled if LLM configured
func (c *Config) validateAI() error {
	// Validate Embeddings
	validProviders := map[string]bool{
		"openai": true,
		"ollama": true,
		"local":  true,
		"custom": true,
	}

	if !validProviders[c.AI.Embeddings.Provider] {
		return fmt.Errorf("ai.embeddings.provider must be openai, ollama, local, or custom, got '%s'",
			c.AI.Embeddings.Provider)
	}

	if c.AI.Embeddings.Model == "" {
		return fmt.Errorf("ai.embeddings.model is required")
	}

	if c.AI.Embeddings.Dim < 1 {
		return fmt.Errorf("ai.embeddings.dim must be at least 1, got %d", c.AI.Embeddings.Dim)
	}

	if c.AI.Embeddings.Dim > 16384 {
		return fmt.Errorf("ai.embeddings.dim should not exceed 16384, got %d", c.AI.Embeddings.Dim)
	}

	if c.AI.Embeddings.TimeoutMs < 100 {
		return fmt.Errorf("ai.embeddings.timeout_ms must be at least 100, got %d", c.AI.Embeddings.TimeoutMs)
	}

	if c.AI.Embeddings.TimeoutMs > 60000 {
		return fmt.Errorf("ai.embeddings.timeout_ms should not exceed 60000 (60s), got %d", c.AI.Embeddings.TimeoutMs)
	}

	// Validate LLM
	if !validProviders[c.AI.LLM.Provider] {
		return fmt.Errorf("ai.llm.provider must be openai, ollama, local, or custom, got '%s'",
			c.AI.LLM.Provider)
	}

	if c.AI.LLM.Model == "" {
		return fmt.Errorf("ai.llm.model is required")
	}

	if c.AI.LLM.Temperature < 0.0 || c.AI.LLM.Temperature > 2.0 {
		return fmt.Errorf("ai.llm.temperature must be between 0.0 and 2.0, got %f", c.AI.LLM.Temperature)
	}

	if c.AI.LLM.MaxTokens < 1 {
		return fmt.Errorf("ai.llm.max_tokens must be at least 1, got %d", c.AI.LLM.MaxTokens)
	}

	if c.AI.LLM.MaxTokens > 32768 {
		return fmt.Errorf("ai.llm.max_tokens should not exceed 32768, got %d", c.AI.LLM.MaxTokens)
	}

	if c.AI.LLM.TimeoutMs < 100 {
		return fmt.Errorf("ai.llm.timeout_ms must be at least 100, got %d", c.AI.LLM.TimeoutMs)
	}

	if c.AI.LLM.TimeoutMs > 120000 { // 2 minutes max for LLM
		return fmt.Errorf("ai.llm.timeout_ms should not exceed 120000 (120s), got %d", c.AI.LLM.TimeoutMs)
	}

	// Validate EntityExtraction
	if c.AI.EntityExtraction.Enabled {
		// Entity extraction requires LLM to be configured
		// Check this after LLM validation to provide a more specific error message
		if c.AI.LLM.Provider == "" || !validProviders[c.AI.LLM.Provider] {
			return fmt.Errorf("ai.entity_extraction.enabled requires ai.llm.provider to be configured (got '%s')",
				c.AI.LLM.Provider)
		}

		if c.AI.LLM.Model == "" {
			return fmt.Errorf("ai.entity_extraction.enabled requires ai.llm.model to be configured")
		}

		if c.AI.EntityExtraction.MaxRetries < 0 {
			return fmt.Errorf("ai.entity_extraction.max_retries must be non-negative, got %d",
				c.AI.EntityExtraction.MaxRetries)
		}

		if c.AI.EntityExtraction.MaxRetries > 10 {
			return fmt.Errorf("ai.entity_extraction.max_retries should not exceed 10, got %d",
				c.AI.EntityExtraction.MaxRetries)
		}
	}

	return nil
}

