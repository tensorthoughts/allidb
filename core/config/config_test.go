package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Verify node config
	if config.Node.ID != "node-1" {
		t.Errorf("Expected node ID 'node-1', got '%s'", config.Node.ID)
	}

	if config.Node.GRPCPort != 7000 {
		t.Errorf("Expected gRPC port 7000, got %d", config.Node.GRPCPort)
	}

	// Verify cluster config
	if config.Cluster.ReplicationFactor != 3 {
		t.Errorf("Expected replication factor 3, got %d", config.Cluster.ReplicationFactor)
	}

	// Verify storage config
	if config.Storage.WAL.MaxFileMB != 128 {
		t.Errorf("Expected WAL max file size 128MB, got %d", config.Storage.WAL.MaxFileMB)
	}

	// Verify index config
	if config.Index.HNSW.M != 16 {
		t.Errorf("Expected HNSW M=16, got %d", config.Index.HNSW.M)
	}

	// Verify query config
	if config.Query.DefaultConsistency != "QUORUM" {
		t.Errorf("Expected default consistency QUORUM, got '%s'", config.Query.DefaultConsistency)
	}

	// Verify security config
	if !config.Security.Auth.Enabled {
		t.Error("Expected auth enabled by default")
	}

	// Verify repair config
	if !config.Repair.ReadRepairEnabled {
		t.Error("Expected read repair enabled by default")
	}

	// Verify AI config
	if config.AI.Embeddings.Provider != "openai" {
		t.Errorf("Expected embeddings provider 'openai', got '%s'", config.AI.Embeddings.Provider)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	config := DefaultConfig()

	// Create a temporary data directory
	tmpDir := t.TempDir()
	config.Node.DataDir = tmpDir

	if err := config.Validate(); err != nil {
		t.Fatalf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidNodeID(t *testing.T) {
	config := DefaultConfig()
	config.Node.ID = ""
	config.Node.DataDir = t.TempDir()

	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for empty node ID")
	}

	if !strings.Contains(err.Error(), "id is required") {
		t.Errorf("Expected error about id, got: %v", err)
	}
}

func TestValidate_InvalidPorts(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid gRPC port
	config.Node.GRPCPort = 0
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid gRPC port")
	}

	// Test same ports
	config.Node.GRPCPort = 7000
	config.Node.HTTPPort = 7000
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error for same gRPC and HTTP ports")
	}
}

func TestValidate_InvalidCluster(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test empty seed nodes
	config.Cluster.SeedNodes = []string{}
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for empty seed nodes")
	}

	// Test invalid replication factor
	config.Cluster.SeedNodes = []string{"node-1:7000"}
	config.Cluster.ReplicationFactor = 0
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid replication factor")
	}

	// Test failure timeout less than gossip interval
	config.Cluster.ReplicationFactor = 3
	config.Cluster.FailureTimeoutMs = 100
	config.Cluster.GossipIntervalMs = 200
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error when failure timeout <= gossip interval")
	}
}

func TestValidate_InvalidStorage(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid WAL max file size
	config.Storage.WAL.MaxFileMB = 0
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid WAL max file size")
	}

	// Test invalid memtable size
	config.Storage.WAL.MaxFileMB = 128
	config.Storage.Memtable.MaxMB = 0
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid memtable size")
	}
}

func TestValidate_InvalidIndex(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid HNSW M
	config.Index.HNSW.M = 1
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid HNSW M")
	}

	// Test ef_construction < M
	config.Index.HNSW.M = 16
	config.Index.HNSW.EfConstruction = 8
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error when ef_construction < M")
	}
}

func TestValidate_InvalidQuery(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid consistency level
	config.Query.DefaultConsistency = "INVALID"
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid consistency level")
	}

	// Test invalid timeout
	config.Query.DefaultConsistency = "QUORUM"
	config.Query.TimeoutMs = 0
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid timeout")
	}
}

func TestValidate_InvalidSecurity(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test TLS enabled without cert file
	config.Security.TLS.Enabled = true
	config.Security.TLS.CertFile = ""
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error when TLS enabled without cert file")
	}

	// Test TLS enabled without key file
	config.Security.TLS.CertFile = "/tmp/cert.pem"
	config.Security.TLS.KeyFile = ""
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error when TLS enabled without key file")
	}
}

func TestValidate_InvalidObservability(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid log level
	config.Observability.LogLevel = "INVALID"
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid log level")
	}
}

func TestValidate_InvalidRepair(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid hinted handoff TTL
	config.Repair.HintedHandoff.TTLMinutes = 0
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid hinted handoff TTL")
	}
}

func TestValidate_InvalidAI(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test invalid embeddings provider
	config.AI.Embeddings.Provider = "invalid"
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid embeddings provider")
	}

	// Test invalid LLM temperature
	config.AI.Embeddings.Provider = "openai"
	config.AI.LLM.Temperature = 3.0
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid LLM temperature")
	}
}

func TestValidate_EntityExtractionRequiresLLM(t *testing.T) {
	config := DefaultConfig()
	config.Node.DataDir = t.TempDir()

	// Test entity extraction enabled without LLM provider
	// Note: LLM provider validation happens first, so we get that error
	config.AI.EntityExtraction.Enabled = true
	config.AI.LLM.Provider = ""
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error when entity extraction enabled without LLM provider")
	}
	// The error will be about LLM provider being required (which is validated first)
	// but entity extraction check also validates this
	if !strings.Contains(err.Error(), "llm.provider") && !strings.Contains(err.Error(), "entity_extraction") {
		t.Errorf("Expected error about LLM provider or entity extraction, got: %v", err)
	}

	// Test entity extraction enabled without LLM model
	config.AI.LLM.Provider = "openai"
	config.AI.LLM.Model = ""
	err = config.Validate()
	if err == nil {
		t.Fatal("Expected error when entity extraction enabled without LLM model")
	}
	// The error will be about LLM model being required (which is validated first)
	// but entity extraction check also validates this
	if !strings.Contains(err.Error(), "llm.model") && !strings.Contains(err.Error(), "entity_extraction") {
		t.Errorf("Expected error about LLM model or entity extraction, got: %v", err)
	}

	// Test entity extraction enabled with LLM configured (should pass)
	config.AI.LLM.Model = "gpt-4o-mini"
	err = config.Validate()
	if err != nil {
		t.Fatalf("Expected no error when entity extraction enabled with LLM configured, got: %v", err)
	}

	// Test entity extraction disabled (should pass even if LLM is not fully configured)
	// Actually, LLM is always required, so this test doesn't make sense.
	// Instead, test that entity extraction can be disabled
	config.AI.EntityExtraction.Enabled = false
	err = config.Validate()
	if err != nil {
		t.Fatalf("Expected no error when entity extraction is disabled, got: %v", err)
	}
}

func TestValidate_DataDirCreation(t *testing.T) {
	config := DefaultConfig()
	
	// Use a non-existent directory
	config.Node.DataDir = t.TempDir() + "/newdir"

	// Should create the directory
	if err := config.Validate(); err != nil {
		t.Fatalf("Expected config to create data dir, got error: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(config.Node.DataDir); err != nil {
		t.Fatalf("Expected data dir to be created, got error: %v", err)
	}
}

func TestValidate_DataDirIsFile(t *testing.T) {
	config := DefaultConfig()
	
	// Create a file instead of directory
	tmpFile := t.TempDir() + "/file"
	os.WriteFile(tmpFile, []byte("test"), 0644)
	config.Node.DataDir = tmpFile

	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error when data_dir is a file")
	}
}


