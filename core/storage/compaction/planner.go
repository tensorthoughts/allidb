package compaction

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

// SSTableInfo holds metadata about an SSTable file.
type SSTableInfo struct {
	Path      string
	Size      int64
	Age       time.Duration // Age since file was created/modified
	RowCount  int64
	Reader    *sstable.Reader
	CreatedAt time.Time
}

// Planner selects SSTables for compaction based on size and age.
type Planner struct {
	sstableDir    string
	sstablePrefix string
	minSize       int64 // Minimum size to consider for compaction
	maxAge        time.Duration // Maximum age before compaction
}

// PlannerConfig holds configuration for the compaction planner.
type PlannerConfig struct {
	SSTableDir    string        // Directory containing SSTable files
	SSTablePrefix string        // Prefix for SSTable files (default: "sstable")
	MinSize       int64         // Minimum size to consider for compaction (default: 1MB)
	MaxAge        time.Duration // Maximum age before compaction (default: 1 hour)
}

// DefaultPlannerConfig returns a default planner configuration.
func DefaultPlannerConfig(sstableDir string) PlannerConfig {
	return PlannerConfig{
		SSTableDir:    sstableDir,
		SSTablePrefix: "sstable",
		MinSize:       1 * 1024 * 1024, // 1MB
		MaxAge:        1 * time.Hour,
	}
}

// NewPlanner creates a new compaction planner.
func NewPlanner(config PlannerConfig) *Planner {
	return &Planner{
		sstableDir:    config.SSTableDir,
		sstablePrefix: config.SSTablePrefix,
		minSize:       config.MinSize,
		maxAge:        config.MaxAge,
	}
}

// ListSSTables lists all SSTable files and returns their metadata.
func (p *Planner) ListSSTables() ([]SSTableInfo, error) {
	pattern := filepath.Join(p.sstableDir, p.sstablePrefix+"-*.sst")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	infos := make([]SSTableInfo, 0, len(matches))
	now := time.Now()

	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue // Skip files we can't stat
		}

		// Read file to create reader
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		reader, err := sstable.NewReader(data)
		if err != nil {
			continue // Skip invalid SSTables
		}

		age := now.Sub(info.ModTime())
		infos = append(infos, SSTableInfo{
			Path:      path,
			Size:      info.Size(),
			Age:       age,
			RowCount:  reader.RowCount(),
			Reader:    reader,
			CreatedAt: info.ModTime(),
		})
	}

	// Sort by size (smallest first) and then by age (oldest first)
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Size != infos[j].Size {
			return infos[i].Size < infos[j].Size
		}
		return infos[i].Age > infos[j].Age
	})

	return infos, nil
}

// PlanCompaction selects SSTables for compaction using size-tiered strategy.
// Returns a list of SSTable paths to compact together.
func (p *Planner) PlanCompaction() ([][]string, error) {
	infos, err := p.ListSSTables()
	if err != nil {
		return nil, err
	}

	if len(infos) < 2 {
		return nil, nil // Need at least 2 SSTables to compact
	}

	// Size-tiered compaction: group SSTables by size tiers
	// Each tier should have roughly 10x the size of the previous tier
	tiers := p.groupBySizeTier(infos)

	// Find tiers that need compaction
	compactions := make([][]string, 0)

	for _, tier := range tiers {
		// If a tier has too many files, compact them
		if len(tier) >= 4 { // Compact when we have 4+ files in a tier
			paths := make([]string, len(tier))
			for i, info := range tier {
				paths[i] = info.Path
			}
			compactions = append(compactions, paths)
			continue
		}

		// Also compact if files are old enough
		oldFiles := make([]string, 0)
		for _, info := range tier {
			if info.Age >= p.maxAge && info.Size >= p.minSize {
				oldFiles = append(oldFiles, info.Path)
			}
		}
		if len(oldFiles) >= 2 {
			compactions = append(compactions, oldFiles)
		}
	}

	return compactions, nil
}

// groupBySizeTier groups SSTables into size tiers.
// Each tier contains files of roughly similar size (within 2x of each other).
func (p *Planner) groupBySizeTier(infos []SSTableInfo) [][]SSTableInfo {
	if len(infos) == 0 {
		return nil
	}

	tiers := make([][]SSTableInfo, 0)
	currentTier := make([]SSTableInfo, 0)
	currentTierSize := int64(0)

	for _, info := range infos {
		// Skip files that are too small
		if info.Size < p.minSize {
			continue
		}

		// Start a new tier if:
		// 1. Current tier is empty, or
		// 2. This file is more than 2x the average size of current tier
		if len(currentTier) == 0 {
			currentTier = append(currentTier, info)
			currentTierSize = info.Size
		} else {
			avgSize := currentTierSize / int64(len(currentTier))
			if info.Size > avgSize*2 {
				// Start new tier
				tiers = append(tiers, currentTier)
				currentTier = []SSTableInfo{info}
				currentTierSize = info.Size
			} else {
				// Add to current tier
				currentTier = append(currentTier, info)
				currentTierSize += info.Size
			}
		}
	}

	// Add the last tier
	if len(currentTier) > 0 {
		tiers = append(tiers, currentTier)
	}

	return tiers
}

// ShouldCompact checks if compaction is needed based on the number and age of SSTables.
func (p *Planner) ShouldCompact() (bool, error) {
	infos, err := p.ListSSTables()
	if err != nil {
		return false, err
	}

	// Need at least 2 SSTables
	if len(infos) < 2 {
		return false, nil
	}

	// Check if any tier has too many files
	tiers := p.groupBySizeTier(infos)
	for _, tier := range tiers {
		if len(tier) >= 4 {
			return true, nil
		}
	}

	// Check if we have old files that should be compacted
	oldCount := 0
	for _, info := range infos {
		if info.Age >= p.maxAge && info.Size >= p.minSize {
			oldCount++
		}
	}

	return oldCount >= 2, nil
}

