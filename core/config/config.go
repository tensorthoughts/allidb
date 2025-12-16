package config

// Config is the root configuration struct for AlliDB.
// It matches the structure of configs/allidb.yaml exactly.
type Config struct {
	Node          NodeConfig          `yaml:"node"`
	Cluster       ClusterConfig       `yaml:"cluster"`
	Storage       StorageConfig       `yaml:"storage"`
	Index         IndexConfig         `yaml:"index"`
	Query         QueryConfig         `yaml:"query"`
	Security      SecurityConfig      `yaml:"security"`
	Observability ObservabilityConfig `yaml:"observability"`
	Repair        RepairConfig        `yaml:"repair"`
	AI            AIConfig            `yaml:"ai"`
}

// NodeConfig holds node-specific configuration.
type NodeConfig struct {
	ID            string `yaml:"id"`
	ListenAddress string `yaml:"listen_address"`
	GRPCPort      int    `yaml:"grpc_port"`
	HTTPPort      int    `yaml:"http_port"`
	DataDir       string `yaml:"data_dir"`
}

// ClusterConfig holds cluster configuration.
type ClusterConfig struct {
	SeedNodes            []string `yaml:"seed_nodes"`
	ReplicationFactor    int      `yaml:"replication_factor"`
	GossipPort           int      `yaml:"gossip_port"`
	GossipIntervalMs     int      `yaml:"gossip_interval_ms"`
	FailureTimeoutMs     int      `yaml:"failure_timeout_ms"`
	CleanupIntervalMs    int      `yaml:"cleanup_interval_ms"`
	Fanout               int      `yaml:"fanout"`
	VirtualNodesPerNode  int      `yaml:"virtual_nodes_per_node"`
}

// StorageConfig holds storage layer configuration.
type StorageConfig struct {
	WAL        WALConfig        `yaml:"wal"`
	Memtable   MemtableConfig   `yaml:"memtable"`
	SSTable    SSTableConfig    `yaml:"sstable"`
	Compaction CompactionConfig `yaml:"compaction"`
}

// WALConfig holds WAL-specific configuration.
type WALConfig struct {
	MaxFileMB int  `yaml:"max_file_mb"`
	Fsync     bool `yaml:"fsync"`
}

// MemtableConfig holds memtable configuration.
type MemtableConfig struct {
	MaxMB int64 `yaml:"max_mb"`
}

// SSTableConfig holds SSTable configuration.
type SSTableConfig struct {
	BlockSizeKB int `yaml:"block_size_kb"`
}

// CompactionConfig holds compaction configuration.
type CompactionConfig struct {
	MaxConcurrent      int `yaml:"max_concurrent"`
	SizeTierThreshold  int `yaml:"size_tier_threshold"`
}

// IndexConfig holds index configuration.
type IndexConfig struct {
	HNSW                 HNSWConfig `yaml:"hnsw"`
	GraphCacheSize       int        `yaml:"graph_cache_size"`
	EntityCacheSize      int        `yaml:"entity_cache_size"`
	ReloadIntervalMs     int        `yaml:"reload_interval_ms"`
	RealtimeIndexEnabled bool       `yaml:"realtime_index_enabled"`
}

// HNSWConfig holds HNSW index configuration.
type HNSWConfig struct {
	M             int `yaml:"m"`
	EfConstruction int `yaml:"ef_construction"`
	EfSearch      int `yaml:"ef_search"`
	ShardCount    int `yaml:"shard_count"`
}

// QueryConfig holds query execution configuration.
type QueryConfig struct {
	DefaultConsistency string `yaml:"default_consistency"`
	MaxGraphHops       int    `yaml:"max_graph_hops"`
	TimeoutMs          int    `yaml:"timeout_ms"`
	DefaultK           int    `yaml:"default_k"`
	MaxCandidates      int    `yaml:"max_candidates"`
}

// SecurityConfig holds security configuration.
type SecurityConfig struct {
	Auth AuthConfig `yaml:"auth"`
	TLS  TLSConfig  `yaml:"tls"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Enabled      bool   `yaml:"enabled"`
	APIKeyHeader string `yaml:"api_key_header"`
	// StaticAPIKeys is an optional list of static API keys loaded from config.
	// Each entry maps an API key to a tenant ID. Intended for development /
	// small deployments; production setups should use an external secret store
	// or identity provider.
	StaticAPIKeys []StaticAPIKeyConfig `yaml:"static_api_keys"`
}

// StaticAPIKeyConfig maps a single API key to a tenant ID.
type StaticAPIKeyConfig struct {
	Key      string `yaml:"key"`
	TenantID string `yaml:"tenant_id"`
}

// TLSConfig holds TLS configuration.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`
}

// ObservabilityConfig holds observability configuration.
type ObservabilityConfig struct {
	LogLevel       string `yaml:"log_level"`
	MetricsEnabled bool   `yaml:"metrics_enabled"`
}

// RepairConfig holds repair system configuration.
type RepairConfig struct {
	ReadRepairEnabled bool              `yaml:"read_repair_enabled"`
	HintedHandoff     HintedHandoffConfig `yaml:"hinted_handoff"`
	AntiEntropy       AntiEntropyConfig   `yaml:"anti_entropy"`
}

// HintedHandoffConfig holds hinted handoff configuration.
type HintedHandoffConfig struct {
	Enabled         bool `yaml:"enabled"`
	MaxHintsPerNode int  `yaml:"max_hints_per_node"`
	MaxSizeMB       int  `yaml:"max_size_mb"`
	TTLMinutes      int  `yaml:"ttl_minutes"`
	FlushIntervalMs int  `yaml:"flush_interval_ms"`
}

// AntiEntropyConfig holds anti-entropy repair configuration.
type AntiEntropyConfig struct {
	Enabled        bool `yaml:"enabled"`
	IntervalMinutes int `yaml:"interval_minutes"`
}

// AIConfig holds AI-related configuration.
type AIConfig struct {
	Embeddings        EmbeddingsConfig        `yaml:"embeddings"`
	LLM               LLMConfig               `yaml:"llm"`
	EntityExtraction  EntityExtractionConfig  `yaml:"entity_extraction"`
}

// EmbeddingsConfig holds embeddings provider configuration.
type EmbeddingsConfig struct {
	Provider  string `yaml:"provider"`  // openai | ollama | local | custom
	Model     string `yaml:"model"`
	Dim       int    `yaml:"dim"`
	TimeoutMs int    `yaml:"timeout_ms"`
}

// LLMConfig holds LLM provider configuration.
type LLMConfig struct {
	Provider   string  `yaml:"provider"`   // openai | ollama | local | custom
	Model      string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
	TimeoutMs   int     `yaml:"timeout_ms"`
}

// EntityExtractionConfig holds entity extraction configuration.
type EntityExtractionConfig struct {
	Enabled   bool `yaml:"enabled"`
	MaxRetries int `yaml:"max_retries"`
}

// DefaultConfig returns a default AlliDB configuration.
// Values match the defaults used throughout the codebase.
func DefaultConfig() Config {
	return Config{
		Node: NodeConfig{
			ID:            "node-1",
			ListenAddress: "0.0.0.0",
			GRPCPort:      7000,
			HTTPPort:      7001,
			DataDir:       "/var/lib/allidb",
		},
		Cluster: ClusterConfig{
			SeedNodes:          []string{"node-1:7000", "node-2:7000"},
			ReplicationFactor:  3,
			GossipPort:         7946,
			GossipIntervalMs:   1000,
			FailureTimeoutMs:   5000,
			CleanupIntervalMs:  60000,
			Fanout:             3,
			VirtualNodesPerNode: 150,
		},
		Storage: StorageConfig{
			WAL: WALConfig{
				MaxFileMB: 128,
				Fsync:     true,
			},
			Memtable: MemtableConfig{
				MaxMB: 256,
			},
			SSTable: SSTableConfig{
				BlockSizeKB: 64,
			},
			Compaction: CompactionConfig{
				MaxConcurrent:     2,
				SizeTierThreshold: 4,
			},
		},
		Index: IndexConfig{
			HNSW: HNSWConfig{
				M:             16,
				EfConstruction: 200,
				EfSearch:      64,
				ShardCount:    16,
			},
			GraphCacheSize:       10000,
			EntityCacheSize:      1000,
			ReloadIntervalMs:     5000,
			RealtimeIndexEnabled: false,
		},
		Query: QueryConfig{
			DefaultConsistency: "QUORUM",
			MaxGraphHops:       2,
			TimeoutMs:          500,
			DefaultK:           10,
			MaxCandidates:      1000,
		},
		Security: SecurityConfig{
			Auth: AuthConfig{
				Enabled:      true,
				APIKeyHeader: "X-ALLIDB-API-KEY",
				StaticAPIKeys: []StaticAPIKeyConfig{
					{
						Key:      "test-api-key",
						TenantID: "tenant1",
					},
				},
			},
			TLS: TLSConfig{
				Enabled:  false,
				CertFile: "",
				KeyFile:  "",
				CAFile:   "",
			},
		},
		Observability: ObservabilityConfig{
			LogLevel:       "INFO",
			MetricsEnabled: true,
		},
		Repair: RepairConfig{
			ReadRepairEnabled: true,
			HintedHandoff: HintedHandoffConfig{
				Enabled:         true,
				MaxHintsPerNode: 10000,
				MaxSizeMB:       1024,
				TTLMinutes:      60,
				FlushIntervalMs: 1000,
			},
			AntiEntropy: AntiEntropyConfig{
				Enabled:        true,
				IntervalMinutes: 1440,
			},
		},
		AI: AIConfig{
			Embeddings: EmbeddingsConfig{
				Provider:  "openai",
				Model:     "text-embedding-3-large",
				Dim:       3072,
				TimeoutMs: 3000,
			},
			LLM: LLMConfig{
				Provider:    "openai",
				Model:       "gpt-4o-mini",
				Temperature: 0.0,
				MaxTokens:   2048,
				TimeoutMs:   5000,
			},
			EntityExtraction: EntityExtractionConfig{
				Enabled:   true,
				MaxRetries: 2,
			},
		},
	}
}

// Validate and all validation methods are implemented in validate.go


