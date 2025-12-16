package entropy

import (
	"context"
	"fmt"
	"sync"

	"github.com/tensorthoughts25/allidb/core/cluster"
)

// TreeProvider provides Merkle trees for partitions.
type TreeProvider interface {
	// GetTree returns the Merkle tree for a partition on a specific node.
	GetTree(ctx context.Context, nodeID cluster.NodeID, partitionID PartitionID) (*MerkleTree, error)
}

// Comparator compares Merkle trees between replicas to find divergent ranges.
type Comparator struct {
	mu sync.RWMutex

	treeProvider TreeProvider
	coordinator  *cluster.Coordinator

	// Statistics
	comparisonsPerformed int64
	divergentRangesFound int64
}

// Config holds configuration for the comparator.
type ComparatorConfig struct {
	TreeProvider TreeProvider
	Coordinator  *cluster.Coordinator
}

// NewComparator creates a new Merkle tree comparator.
func NewComparator(config ComparatorConfig) *Comparator {
	return &Comparator{
		treeProvider: config.TreeProvider,
		coordinator:  config.Coordinator,
	}
}

// ComparePartition compares Merkle trees for a partition across all replicas.
// Returns divergent key ranges that need repair.
func (c *Comparator) ComparePartition(ctx context.Context, partitionID PartitionID) ([]DivergentRange, error) {
	if c.coordinator == nil {
		return nil, fmt.Errorf("coordinator not set")
	}

	// Get replicas for this partition using the ring via coordinator
	replicas := c.coordinator.GetAliveReplicas(string(partitionID))
	
	if len(replicas) < 2 {
		// Need at least 2 replicas to compare
		return []DivergentRange{}, nil
	}

	// Fetch Merkle trees from all replicas
	trees := make(map[cluster.NodeID]*MerkleTree)
	var fetchErr error
	var fetchMu sync.Mutex
	var fetchWg sync.WaitGroup

	for _, replica := range replicas {
		fetchWg.Add(1)
		go func(nodeID cluster.NodeID) {
			defer fetchWg.Done()

			tree, err := c.treeProvider.GetTree(ctx, nodeID, partitionID)
			if err != nil {
				fetchMu.Lock()
				if fetchErr == nil {
					fetchErr = err
				}
				fetchMu.Unlock()
				return
			}

			fetchMu.Lock()
			trees[nodeID] = tree
			fetchMu.Unlock()
		}(replica)
	}

	fetchWg.Wait()

	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch trees: %w", fetchErr)
	}

	if len(trees) < 2 {
		return []DivergentRange{}, nil
	}

	// Compare trees pairwise to find divergent ranges
	divergentRanges := c.findDivergentRanges(trees, partitionID)

	// Update statistics
	c.mu.Lock()
	c.comparisonsPerformed++
	c.divergentRangesFound += int64(len(divergentRanges))
	c.mu.Unlock()

	return divergentRanges, nil
}

// findDivergentRanges compares trees and finds divergent key ranges.
func (c *Comparator) findDivergentRanges(trees map[cluster.NodeID]*MerkleTree, partitionID PartitionID) []DivergentRange {
	// Find the majority tree (most common root hash)
	majorityTree := c.findMajorityTree(trees)
	if majorityTree == nil {
		// No majority, all ranges are potentially divergent
		return c.allRangesDivergent(trees, partitionID)
	}

	// Compare all trees against the majority tree
	divergentRanges := make([]DivergentRange, 0)
	seenRanges := make(map[string]bool)

	for nodeID, tree := range trees {
		if tree == majorityTree {
			continue // Skip majority tree
		}

		// Find divergent ranges
		ranges := majorityTree.FindDivergentRanges(tree)
		for _, keyRange := range ranges {
			// Create unique key for this range
			rangeKey := fmt.Sprintf("%s:%s:%s", keyRange.Start, keyRange.End, nodeID)
			if seenRanges[rangeKey] {
				continue
			}
			seenRanges[rangeKey] = true

			divergentRange := DivergentRange{
				PartitionID: partitionID,
				KeyRange:    keyRange,
				DivergentNodes: []cluster.NodeID{nodeID},
				CorrectNode:    c.findCorrectNode(trees, majorityTree),
			}
			divergentRanges = append(divergentRanges, divergentRange)
		}
	}

	return divergentRanges
}

// findMajorityTree finds the tree that appears in the majority of replicas.
func (c *Comparator) findMajorityTree(trees map[cluster.NodeID]*MerkleTree) *MerkleTree {
	if len(trees) == 0 {
		return nil
	}

	// Count trees by root hash
	hashCounts := make(map[string]int)
	hashToTree := make(map[string]*MerkleTree)

	for _, tree := range trees {
		if tree == nil || tree.Root == nil {
			continue
		}
		hashKey := string(tree.Root.Hash)
		hashCounts[hashKey]++
		hashToTree[hashKey] = tree
	}

	// Find hash with maximum count
	maxCount := 0
	var majorityHash string
	for hash, count := range hashCounts {
		if count > maxCount {
			maxCount = count
			majorityHash = hash
		}
	}

	// Check if we have a majority (more than half)
	if maxCount > len(trees)/2 {
		return hashToTree[majorityHash]
	}

	return nil
}

// allRangesDivergent marks all ranges as divergent when no majority exists.
func (c *Comparator) allRangesDivergent(trees map[cluster.NodeID]*MerkleTree, partitionID PartitionID) []DivergentRange {
	divergentRanges := make([]DivergentRange, 0)
	
	// Get all key ranges from all trees
	allRanges := make(map[string]KeyRange)
	for _, tree := range trees {
		if tree != nil {
			for _, keyRange := range tree.GetKeyRanges() {
				rangeKey := fmt.Sprintf("%s:%s", keyRange.Start, keyRange.End)
				allRanges[rangeKey] = keyRange
			}
		}
	}

	// Mark all ranges as divergent
	for _, keyRange := range allRanges {
		divergentRange := DivergentRange{
			PartitionID: partitionID,
			KeyRange:    keyRange,
			DivergentNodes: make([]cluster.NodeID, 0),
		}
		divergentRanges = append(divergentRanges, divergentRange)
	}

	return divergentRanges
}

// findCorrectNode finds a node that has the correct tree.
func (c *Comparator) findCorrectNode(trees map[cluster.NodeID]*MerkleTree, correctTree *MerkleTree) cluster.NodeID {
	for nodeID, tree := range trees {
		if tree == correctTree {
			return nodeID
		}
	}
	// Return first node if no match found
	for nodeID := range trees {
		return nodeID
	}
	return cluster.NodeID("")
}

// DivergentRange represents a divergent key range that needs repair.
type DivergentRange struct {
	PartitionID    PartitionID
	KeyRange       KeyRange
	DivergentNodes []cluster.NodeID
	CorrectNode    cluster.NodeID
}

// GetComparisonsPerformed returns the number of comparisons performed.
func (c *Comparator) GetComparisonsPerformed() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.comparisonsPerformed
}

// GetDivergentRangesFound returns the number of divergent ranges found.
func (c *Comparator) GetDivergentRangesFound() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.divergentRangesFound
}

