package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadConfig_ValidYAML(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	
	yamlContent := `
node:
  id: "test-node"
  listen_address: "127.0.0.1"
  grpc_port: 8000
  http_port: 8001
  data_dir: "` + tmpDir + `"
cluster:
  seed_nodes:
    - "node-1:8000"
  replication_factor: 2
storage:
  wal:
    max_file_mb: 64
    fsync: false
index:
  hnsw:
    m: 32
query:
  default_consistency: "ONE"
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	config, err := LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify YAML values override defaults
	if config.Node.ID != "test-node" {
		t.Errorf("Expected node ID 'test-node', got '%s'", config.Node.ID)
	}

	if config.Node.GRPCPort != 8000 {
		t.Errorf("Expected gRPC port 8000, got %d", config.Node.GRPCPort)
	}

	if config.Storage.WAL.MaxFileMB != 64 {
		t.Errorf("Expected WAL max file size 64MB, got %d", config.Storage.WAL.MaxFileMB)
	}

	if config.Storage.WAL.Fsync {
		t.Error("Expected fsync to be false (overridden from YAML)")
	}

	if config.Index.HNSW.M != 32 {
		t.Errorf("Expected HNSW M=32, got %d", config.Index.HNSW.M)
	}

	if config.Query.DefaultConsistency != "ONE" {
		t.Errorf("Expected consistency ONE, got '%s'", config.Query.DefaultConsistency)
	}

	// Verify defaults are still applied for fields not in YAML
	if config.Cluster.GossipIntervalMs != 1000 {
		t.Errorf("Expected default gossip interval 1000ms, got %d", config.Cluster.GossipIntervalMs)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")
	
	invalidYAML := `
node:
  id: "test-node"
  grpc_port: invalid
`

	if err := os.WriteFile(yamlFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	_, err := LoadConfig(yamlFile)
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}

	if !strings.Contains(err.Error(), "failed to parse YAML") {
		t.Errorf("Expected YAML parse error, got: %v", err)
	}
}

func TestLoadConfig_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid-config.yaml")
	
	// Use invalid port numbers that will fail validation
	invalidConfig := `
node:
  id: "test-node"
  listen_address: "0.0.0.0"
  grpc_port: 70000
  http_port: 7001
  data_dir: "` + tmpDir + `"
`

	if err := os.WriteFile(yamlFile, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	_, err := LoadConfig(yamlFile)
	if err == nil {
		t.Fatal("Expected validation error")
	}

	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "partial.yaml")
	
	// Only override a few fields
	partialYAML := `
node:
  id: "partial-node"
  data_dir: "` + tmpDir + `"
storage:
  wal:
    max_file_mb: 256
`

	if err := os.WriteFile(yamlFile, []byte(partialYAML), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	config, err := LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify overridden values
	if config.Node.ID != "partial-node" {
		t.Errorf("Expected node ID 'partial-node', got '%s'", config.Node.ID)
	}

	if config.Storage.WAL.MaxFileMB != 256 {
		t.Errorf("Expected WAL max file size 256MB, got %d", config.Storage.WAL.MaxFileMB)
	}

	// Verify defaults are still applied
	if config.Node.GRPCPort != 7000 {
		t.Errorf("Expected default gRPC port 7000, got %d", config.Node.GRPCPort)
	}

	if config.Cluster.ReplicationFactor != 3 {
		t.Errorf("Expected default replication factor 3, got %d", config.Cluster.ReplicationFactor)
	}
}

func TestRedactSecrets(t *testing.T) {
	config := DefaultConfig()
	config.Security.Auth.APIKeyHeader = "X-SECRET-KEY"
	config.Security.TLS.CertFile = "/path/to/cert.pem"
	config.Security.TLS.KeyFile = "/path/to/key.pem"
	config.Security.TLS.CAFile = "/path/to/ca.pem"

	redacted := RedactSecrets(&config)

	// Verify secrets are redacted
	if redacted.Security.Auth.APIKeyHeader != "[REDACTED]" {
		t.Errorf("Expected API key header to be redacted, got '%s'", redacted.Security.Auth.APIKeyHeader)
	}

	if redacted.Security.TLS.CertFile != "[REDACTED]" {
		t.Errorf("Expected cert file to be redacted, got '%s'", redacted.Security.TLS.CertFile)
	}

	if redacted.Security.TLS.KeyFile != "[REDACTED]" {
		t.Errorf("Expected key file to be redacted, got '%s'", redacted.Security.TLS.KeyFile)
	}

	if redacted.Security.TLS.CAFile != "[REDACTED]" {
		t.Errorf("Expected CA file to be redacted, got '%s'", redacted.Security.TLS.CAFile)
	}

	// Verify non-secret fields are preserved
	if redacted.Node.ID != config.Node.ID {
		t.Errorf("Expected node ID to be preserved, got '%s'", redacted.Node.ID)
	}
}

func TestConfig_String(t *testing.T) {
	config := DefaultConfig()
	config.Security.Auth.APIKeyHeader = "X-SECRET-KEY"
	config.Security.TLS.CertFile = "/path/to/cert.pem"

	str := config.String()

	// Verify secrets are not in the string representation
	if strings.Contains(str, "X-SECRET-KEY") {
		t.Error("String representation should not contain API key header")
	}

	if strings.Contains(str, "/path/to/cert.pem") {
		t.Error("String representation should not contain cert file path")
	}

	// Verify non-secret fields are present
	if !strings.Contains(str, config.Node.ID) {
		t.Error("String representation should contain node ID")
	}
}

func TestConfig_ToYAML(t *testing.T) {
	config := DefaultConfig()
	config.Security.Auth.APIKeyHeader = "X-SECRET-KEY"
	config.Security.TLS.CertFile = "/path/to/cert.pem"

	yamlStr, err := config.ToYAML()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify secrets are redacted in YAML
	if strings.Contains(yamlStr, "X-SECRET-KEY") {
		t.Error("YAML representation should not contain API key header")
	}

	if strings.Contains(yamlStr, "/path/to/cert.pem") {
		t.Error("YAML representation should not contain cert file path")
	}

	// Verify YAML is valid
	if !strings.Contains(yamlStr, "node:") {
		t.Error("YAML should contain node section")
	}
}

func TestLoadConfig_BooleanOverride(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "bool-override.yaml")
	
	// Explicitly set fsync to false
	yamlContent := `
node:
  id: "test-node"
  listen_address: "0.0.0.0"
  grpc_port: 7000
  http_port: 7001
  data_dir: "` + tmpDir + `"
storage:
  wal:
    fsync: false
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	config, err := LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify boolean override works
	if config.Storage.WAL.Fsync {
		t.Error("Expected fsync to be false (overridden from YAML)")
	}
}

func TestLoadConfig_EmptyYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "empty.yaml")
	
	// Empty YAML should use all defaults, but we need to override data_dir
	// to avoid permission issues with /var/lib/allidb
	emptyYAML := `
node:
  data_dir: "` + tmpDir + `"
`

	if err := os.WriteFile(yamlFile, []byte(emptyYAML), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	config, err := LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify defaults are used for fields not in YAML
	defaultConfig := DefaultConfig()
	if config.Node.ID != defaultConfig.Node.ID {
		t.Errorf("Expected default node ID, got '%s'", config.Node.ID)
	}

	if config.Storage.WAL.MaxFileMB != defaultConfig.Storage.WAL.MaxFileMB {
		t.Errorf("Expected default WAL max file size, got %d", config.Storage.WAL.MaxFileMB)
	}

	// Verify data_dir was overridden
	if config.Node.DataDir != tmpDir {
		t.Errorf("Expected data_dir to be overridden, got '%s'", config.Node.DataDir)
	}
}

func TestLoadConfig_EnvOverridesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	
	// Create YAML with some values
	yamlContent := `
node:
  id: "yaml-node"
  grpc_port: 8000
  data_dir: "` + tmpDir + `"
storage:
  wal:
    max_file_mb: 256
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Set environment variables that should override YAML
	os.Setenv("ALLIDB_NODE_ID", "env-node")
	os.Setenv("ALLIDB_STORAGE_WAL_MAX_FILE_MB", "512")
	defer func() {
		os.Unsetenv("ALLIDB_NODE_ID")
		os.Unsetenv("ALLIDB_STORAGE_WAL_MAX_FILE_MB")
	}()

	config, err := LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify env vars override YAML
	if config.Node.ID != "env-node" {
		t.Errorf("Expected node ID 'env-node' (from env), got '%s'", config.Node.ID)
	}

	if config.Storage.WAL.MaxFileMB != 512 {
		t.Errorf("Expected WAL max file size 512MB (from env), got %d", config.Storage.WAL.MaxFileMB)
	}

	// Verify YAML values are still used for fields not in env
	if config.Node.GRPCPort != 8000 {
		t.Errorf("Expected gRPC port 8000 (from YAML), got %d", config.Node.GRPCPort)
	}
}

func TestLoadConfig_ActualYAMLFile(t *testing.T) {
	// Test loading the actual configs/allidb.yaml file if it exists
	// Try multiple possible paths
	possiblePaths := []string{
		"configs/allidb.yaml",
		"../configs/allidb.yaml",
		"../../configs/allidb.yaml",
	}
	
	var yamlFile string
	var found bool
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			yamlFile = path
			found = true
			break
		}
	}
	
	if !found {
		t.Skip("configs/allidb.yaml not found in any expected location, skipping test")
	}

	// Read and modify the YAML to use a temp directory for data_dir
	// to avoid permission issues
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		t.Fatalf("Failed to read YAML file: %v", err)
	}

	tmpDir := t.TempDir()

	// Detect the current data_dir value from YAML so the test is resilient to changes.
	type nodeCfg struct {
		Node struct {
			DataDir string `yaml:"data_dir"`
		} `yaml:"node"`
	}
	var nc nodeCfg
	if err := yaml.Unmarshal(data, &nc); err != nil {
		t.Fatalf("Failed to parse YAML for node.data_dir: %v", err)
	}
	origDir := nc.Node.DataDir
	if origDir == "" {
		origDir = "/var/lib/allidb"
	}

	// Replace the data_dir in the YAML content
	yamlContent := strings.ReplaceAll(string(data), origDir, tmpDir)

	// Write to a temporary file
	tmpYAML := filepath.Join(tmpDir, "test-config.yaml")
	if err := os.WriteFile(tmpYAML, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp YAML: %v", err)
	}

	config, err := LoadConfig(tmpYAML)
	if err != nil {
		t.Fatalf("Failed to load actual YAML file %s: %v", yamlFile, err)
	}

	// Verify it loaded successfully
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	// Verify some expected values from the actual file
	if config.Node.ID == "" {
		t.Error("Node ID should not be empty")
	}

	if config.Cluster.ReplicationFactor == 0 {
		t.Error("Replication factor should not be zero")
	}

	// Verify data_dir was updated (unless overridden via env)
	if envDir := os.Getenv("ALLIDB_NODE_DATA_DIR"); envDir == "" {
		if config.Node.DataDir != tmpDir {
			t.Errorf("Expected data_dir to be %s, got %s", tmpDir, config.Node.DataDir)
		}
	}
}
