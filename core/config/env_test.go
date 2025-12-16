package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestApplyEnvOverrides_String(t *testing.T) {
	config := DefaultConfig()
	
	// Set environment variable
	os.Setenv("ALLIDB_NODE_ID", "env-node-1")
	defer os.Unsetenv("ALLIDB_NODE_ID")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Node.ID != "env-node-1" {
		t.Errorf("Expected node ID 'env-node-1', got '%s'", config.Node.ID)
	}
}

func TestApplyEnvOverrides_Int(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_NODE_GRPC_PORT", "9000")
	defer os.Unsetenv("ALLIDB_NODE_GRPC_PORT")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Node.GRPCPort != 9000 {
		t.Errorf("Expected gRPC port 9000, got %d", config.Node.GRPCPort)
	}
}

func TestApplyEnvOverrides_Bool(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_STORAGE_WAL_FSYNC", "false")
	defer os.Unsetenv("ALLIDB_STORAGE_WAL_FSYNC")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Storage.WAL.Fsync {
		t.Error("Expected fsync to be false")
	}
}

func TestApplyEnvOverrides_Float(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_AI_LLM_TEMPERATURE", "0.7")
	defer os.Unsetenv("ALLIDB_AI_LLM_TEMPERATURE")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.AI.LLM.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", config.AI.LLM.Temperature)
	}
}

func TestApplyEnvOverrides_NestedFields(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_AI_LLM_PROVIDER", "ollama")
	defer os.Unsetenv("ALLIDB_AI_LLM_PROVIDER")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.AI.LLM.Provider != "ollama" {
		t.Errorf("Expected provider 'ollama', got '%s'", config.AI.LLM.Provider)
	}
}

func TestApplyEnvOverrides_MultipleFields(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_NODE_ID", "multi-node")
	os.Setenv("ALLIDB_NODE_GRPC_PORT", "8000")
	os.Setenv("ALLIDB_STORAGE_WAL_MAX_FILE_MB", "512")
	defer func() {
		os.Unsetenv("ALLIDB_NODE_ID")
		os.Unsetenv("ALLIDB_NODE_GRPC_PORT")
		os.Unsetenv("ALLIDB_STORAGE_WAL_MAX_FILE_MB")
	}()

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Node.ID != "multi-node" {
		t.Errorf("Expected node ID 'multi-node', got '%s'", config.Node.ID)
	}

	if config.Node.GRPCPort != 8000 {
		t.Errorf("Expected gRPC port 8000, got %d", config.Node.GRPCPort)
	}

	if config.Storage.WAL.MaxFileMB != 512 {
		t.Errorf("Expected WAL max file size 512MB, got %d", config.Storage.WAL.MaxFileMB)
	}
}

func TestApplyEnvOverrides_IgnoreUnknownKeys(t *testing.T) {
	config := DefaultConfig()
	
	// Set an unknown environment variable
	os.Setenv("ALLIDB_UNKNOWN_FIELD", "value")
	os.Setenv("ALLIDB_NODE_ID", "test-node")
	defer func() {
		os.Unsetenv("ALLIDB_UNKNOWN_FIELD")
		os.Unsetenv("ALLIDB_NODE_ID")
	}()

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error for unknown key, got: %v", err)
	}

	// Verify known field was set
	if config.Node.ID != "test-node" {
		t.Errorf("Expected node ID 'test-node', got '%s'", config.Node.ID)
	}
}

func TestApplyEnvOverrides_IgnoreNonPrefix(t *testing.T) {
	config := DefaultConfig()
	
	// Set environment variable without prefix
	os.Setenv("NODE_ID", "should-be-ignored")
	os.Setenv("ALLIDB_NODE_ID", "should-be-used")
	defer func() {
		os.Unsetenv("NODE_ID")
		os.Unsetenv("ALLIDB_NODE_ID")
	}()

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Node.ID != "should-be-used" {
		t.Errorf("Expected node ID 'should-be-used', got '%s'", config.Node.ID)
	}
}

func TestApplyEnvOverrides_TypeMismatch_Int(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_NODE_GRPC_PORT", "not-a-number")
	defer os.Unsetenv("ALLIDB_NODE_GRPC_PORT")

	err := ApplyEnvOverrides(&config)
	if err == nil {
		t.Fatal("Expected error for invalid int value")
	}

	if !contains(err.Error(), "invalid int value") {
		t.Errorf("Expected 'invalid int value' error, got: %v", err)
	}
}

func TestApplyEnvOverrides_TypeMismatch_Bool(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_STORAGE_WAL_FSYNC", "not-a-bool")
	defer os.Unsetenv("ALLIDB_STORAGE_WAL_FSYNC")

	err := ApplyEnvOverrides(&config)
	if err == nil {
		t.Fatal("Expected error for invalid bool value")
	}

	if !contains(err.Error(), "invalid bool value") {
		t.Errorf("Expected 'invalid bool value' error, got: %v", err)
	}
}

func TestApplyEnvOverrides_TypeMismatch_Float(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_AI_LLM_TEMPERATURE", "not-a-float")
	defer os.Unsetenv("ALLIDB_AI_LLM_TEMPERATURE")

	err := ApplyEnvOverrides(&config)
	if err == nil {
		t.Fatal("Expected error for invalid float value")
	}

	if !contains(err.Error(), "invalid float value") {
		t.Errorf("Expected 'invalid float value' error, got: %v", err)
	}
}

func TestApplyEnvOverrides_Slice(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_CLUSTER_SEED_NODES", "node1:7000,node2:7000,node3:7000")
	defer os.Unsetenv("ALLIDB_CLUSTER_SEED_NODES")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := []string{"node1:7000", "node2:7000", "node3:7000"}
	if !reflect.DeepEqual(config.Cluster.SeedNodes, expected) {
		t.Errorf("Expected seed nodes %v, got %v", expected, config.Cluster.SeedNodes)
	}
}

func TestApplyEnvOverrides_SliceWithSpaces(t *testing.T) {
	config := DefaultConfig()
	
	os.Setenv("ALLIDB_CLUSTER_SEED_NODES", "node1:7000, node2:7000 , node3:7000")
	defer os.Unsetenv("ALLIDB_CLUSTER_SEED_NODES")

	if err := ApplyEnvOverrides(&config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := []string{"node1:7000", "node2:7000", "node3:7000"}
	if !reflect.DeepEqual(config.Cluster.SeedNodes, expected) {
		t.Errorf("Expected seed nodes %v, got %v", expected, config.Cluster.SeedNodes)
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ID", "id"},
		{"GRPCPort", "grpc_port"},
		{"ListenAddress", "listen_address"},
		{"MaxFileMB", "max_file_mb"},
		{"EfConstruction", "ef_construction"},
		{"APIKeyHeader", "api_key_header"},
		{"TTLMinutes", "ttl_minutes"},
		{"IntervalMinutes", "interval_minutes"},
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestBuildFieldMap(t *testing.T) {
	fieldMap := buildFieldMap(reflect.TypeOf(Config{}))

	// Check some expected mappings
	expectedMappings := map[string]string{
		"node_id":              "Node.ID",
		"node_grpc_port":       "Node.GRPCPort",
		"storage_wal_max_file_mb": "Storage.WAL.MaxFileMB",
		"ai_llm_provider":      "AI.LLM.Provider",
		"cluster_seed_nodes":   "Cluster.SeedNodes",
	}

	for envKey, expectedPath := range expectedMappings {
		fieldInfo, ok := fieldMap[envKey]
		if !ok {
			t.Errorf("Expected field map to contain '%s'", envKey)
			continue
		}

		actualPath := strings.Join(fieldInfo.path, ".")
		if actualPath != expectedPath {
			t.Errorf("For env key '%s', expected path '%s', got '%s'", envKey, expectedPath, actualPath)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		indexOfSubstring(s, substr) >= 0)))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

