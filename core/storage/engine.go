package storage

import (
	"time"

	"github.com/tensorthoughts25/allidb/core/config"
	"github.com/tensorthoughts25/allidb/core/storage/compaction"
	"github.com/tensorthoughts25/allidb/core/storage/wal"
)

// EngineConfig holds configuration for the storage engine.
// This wires together WAL, memtable, and compaction configuration.
type EngineConfig struct {
	// WAL configuration
	WALDir      string
	WALMaxFileMB int
	WALFsync    bool

	// Memtable configuration
	MemtableMaxMB int64

	// SSTable directory
	SSTableDir string

	// Compaction configuration
	CompactionMaxConcurrent int
}

// NewEngineConfigFromConfig creates an EngineConfig from the global config.
func NewEngineConfigFromConfig(cfg config.StorageConfig, dataDir string) EngineConfig {
	return EngineConfig{
		WALDir:            dataDir + "/wal",
		WALMaxFileMB:      cfg.WAL.MaxFileMB,
		WALFsync:          cfg.WAL.Fsync,
		MemtableMaxMB:     cfg.Memtable.MaxMB,
		SSTableDir:        dataDir + "/sstables",
		CompactionMaxConcurrent: cfg.Compaction.MaxConcurrent,
	}
}

// WALConfig creates a WAL config from engine config.
func (e EngineConfig) WALConfig(entryFactory func() wal.WALEntry) wal.Config {
	return wal.Config{
		Dir:         e.WALDir,
		MaxFileSize: int64(e.WALMaxFileMB) * 1024 * 1024,
		FilePrefix:  "wal",
		Fsync:       e.WALFsync,
		EntryFactory: entryFactory,
	}
}

// MemtableConfig creates a memtable config from engine config.
func (e EngineConfig) MemtableConfig() int64 {
	return e.MemtableMaxMB * 1024 * 1024 // Convert MB to bytes
}

// CompactionConfig creates a compaction config from engine config.
func (e EngineConfig) CompactionConfig() compaction.Config {
	return compaction.Config{
		PlannerConfig: compaction.PlannerConfig{
			SSTableDir:    e.SSTableDir,
			SSTablePrefix: "sstable",
			MinSize:       1 * 1024 * 1024, // 1MB
			MaxAge:        1 * time.Hour,
		},
		ExecutorConfig: compaction.ExecutorConfig{
			SSTableDir: e.SSTableDir,
		},
		CheckInterval: 30 * time.Second,
		MaxConcurrent: e.CompactionMaxConcurrent,
		MaxIOPS:       1000,
		MaxBandwidth:  100 * 1024 * 1024, // 100MB/s
	}
}

