package entropy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// PartitionID represents a partition identifier.
type PartitionID string

// MerkleNode represents a node in the Merkle tree.
type MerkleNode struct {
	Hash      []byte
	Left      *MerkleNode
	Right     *MerkleNode
	KeyRange  *KeyRange // Only set for leaf nodes
	DataHash  []byte    // Only set for leaf nodes
}

// KeyRange represents a range of keys.
type KeyRange struct {
	Start string
	End   string
}

// MerkleTree represents a Merkle tree for a partition.
type MerkleTree struct {
	PartitionID PartitionID
	Root        *MerkleNode
	KeyRanges   []KeyRange
	Depth       int
}

// MerkleBuilder builds Merkle trees for partitions.
type MerkleBuilder struct {
	partitionID PartitionID
}

// NewMerkleBuilder creates a new Merkle tree builder.
func NewMerkleBuilder(partitionID PartitionID) *MerkleBuilder {
	return &MerkleBuilder{
		partitionID: partitionID,
	}
}

// BuildTree builds a Merkle tree from key-value pairs.
// Keys should be sorted for consistent tree structure.
func (mb *MerkleBuilder) BuildTree(keyValues map[string][]byte) (*MerkleTree, error) {
	if len(keyValues) == 0 {
		return &MerkleTree{
			PartitionID: mb.partitionID,
			Root:        nil,
			KeyRanges:   []KeyRange{},
			Depth:       0,
		}, nil
	}

	// Sort keys for consistent tree structure
	keys := make([]string, 0, len(keyValues))
	for key := range keyValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Create leaf nodes
	leaves := make([]*MerkleNode, 0, len(keys))
	keyRanges := make([]KeyRange, 0, len(keys))

	for i, key := range keys {
		value := keyValues[key]
		dataHash := hashData(key, value)
		
		// Determine key range for this leaf
		var keyRange KeyRange
		if i == 0 {
			keyRange.Start = key
		} else {
			keyRange.Start = keys[i-1]
		}
		keyRange.End = key

		leaf := &MerkleNode{
			Hash:     dataHash,
			DataHash: dataHash,
			KeyRange: &keyRange,
		}
		leaves = append(leaves, leaf)
		keyRanges = append(keyRanges, keyRange)
	}

	// Build tree from leaves
	root, depth := mb.buildTreeFromLeaves(leaves)

	return &MerkleTree{
		PartitionID: mb.partitionID,
		Root:        root,
		KeyRanges:   keyRanges,
		Depth:       depth,
	}, nil
}

// buildTreeFromLeaves builds a binary Merkle tree from leaf nodes.
func (mb *MerkleBuilder) buildTreeFromLeaves(leaves []*MerkleNode) (*MerkleNode, int) {
	if len(leaves) == 0 {
		return nil, 0
	}

	if len(leaves) == 1 {
		return leaves[0], 1
	}

	// Build tree level by level
	currentLevel := leaves
	depth := 1

	for len(currentLevel) > 1 {
		nextLevel := make([]*MerkleNode, 0, (len(currentLevel)+1)/2)

		for i := 0; i < len(currentLevel); i += 2 {
			left := currentLevel[i]
			var right *MerkleNode
			
			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			} else {
				// Odd number of nodes, duplicate the last one
				right = left
			}

			// Combine hashes
			combinedHash := hashNodes(left.Hash, right.Hash)
			
			// Determine key range for this node
			var keyRange *KeyRange
			if left.KeyRange != nil && right.KeyRange != nil {
				keyRange = &KeyRange{
					Start: left.KeyRange.Start,
					End:   right.KeyRange.End,
				}
			} else if left.KeyRange != nil {
				keyRange = left.KeyRange
			} else if right.KeyRange != nil {
				keyRange = right.KeyRange
			}

			node := &MerkleNode{
				Hash:     combinedHash,
				Left:     left,
				Right:    right,
				KeyRange: keyRange,
			}
			nextLevel = append(nextLevel, node)
		}

		currentLevel = nextLevel
		depth++
	}

	return currentLevel[0], depth
}

// GetRootHash returns the root hash of the tree.
func (mt *MerkleTree) GetRootHash() []byte {
	if mt.Root == nil {
		return nil
	}
	return mt.Root.Hash
}

// GetKeyRanges returns all key ranges in the tree.
func (mt *MerkleTree) GetKeyRanges() []KeyRange {
	return mt.KeyRanges
}

// FindDivergentRanges compares this tree with another and returns divergent key ranges.
func (mt *MerkleTree) FindDivergentRanges(other *MerkleTree) []KeyRange {
	if mt.Root == nil && other.Root == nil {
		return []KeyRange{}
	}

	if mt.Root == nil || other.Root == nil {
		// One tree is empty, entire partition is divergent
		if mt.Root != nil {
			return mt.GetKeyRanges()
		}
		return other.GetKeyRanges()
	}

	// Compare root hashes
	if bytesEqual(mt.Root.Hash, other.Root.Hash) {
		// Trees are identical
		return []KeyRange{}
	}

	// Trees differ, find divergent ranges
	return mt.compareNodes(mt.Root, other.Root)
}

// compareNodes recursively compares two nodes and returns divergent key ranges.
func (mt *MerkleTree) compareNodes(node1, node2 *MerkleNode) []KeyRange {
	if node1 == nil && node2 == nil {
		return []KeyRange{}
	}

	if node1 == nil || node2 == nil {
		// One node is nil, entire subtree is divergent
		if node1 != nil && node1.KeyRange != nil {
			return []KeyRange{*node1.KeyRange}
		}
		if node2 != nil && node2.KeyRange != nil {
			return []KeyRange{*node2.KeyRange}
		}
		return []KeyRange{}
	}

	// If hashes match, subtrees are identical
	if bytesEqual(node1.Hash, node2.Hash) {
		return []KeyRange{}
	}

	// If both are leaves, this range is divergent
	if node1.Left == nil && node1.Right == nil && node2.Left == nil && node2.Right == nil {
		if node1.KeyRange != nil {
			return []KeyRange{*node1.KeyRange}
		}
		return []KeyRange{}
	}

	// Recurse into children
	divergentRanges := make([]KeyRange, 0)
	
	// Compare left subtrees
	leftRanges := mt.compareNodes(node1.Left, node2.Left)
	divergentRanges = append(divergentRanges, leftRanges...)

	// Compare right subtrees
	rightRanges := mt.compareNodes(node1.Right, node2.Right)
	divergentRanges = append(divergentRanges, rightRanges...)

	return divergentRanges
}

// hashData hashes a key-value pair.
func hashData(key string, value []byte) []byte {
	h := sha256.New()
	h.Write([]byte(key))
	h.Write([]byte{0}) // Separator
	h.Write(value)
	return h.Sum(nil)
}

// hashNodes hashes two node hashes together.
func hashNodes(hash1, hash2 []byte) []byte {
	h := sha256.New()
	h.Write(hash1)
	h.Write(hash2)
	return h.Sum(nil)
}

// bytesEqual compares two byte slices for equality.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// wireMerkleNode is the on-the-wire representation for serialization.
type wireMerkleNode struct {
	Hash     []byte         `json:"hash"`
	DataHash []byte         `json:"data_hash,omitempty"`
	KeyRange *KeyRange      `json:"key_range,omitempty"`
	Left     *wireMerkleNode `json:"left,omitempty"`
	Right    *wireMerkleNode `json:"right,omitempty"`
}

type wireMerkleTree struct {
	PartitionID PartitionID     `json:"partition_id"`
	Depth       int             `json:"depth"`
	Root        *wireMerkleNode `json:"root,omitempty"`
}

// Serialize serializes the full Merkle tree structure to JSON.
func (mt *MerkleTree) Serialize() ([]byte, error) {
	wireTree := wireMerkleTree{
		PartitionID: mt.PartitionID,
		Depth:       mt.Depth,
		Root:        toWireNode(mt.Root),
	}
	return json.Marshal(&wireTree)
}

// DeserializeMerkleTree deserializes a Merkle tree from JSON.
func DeserializeMerkleTree(data []byte, partitionID PartitionID) (*MerkleTree, error) {
	if len(data) == 0 {
		return &MerkleTree{PartitionID: partitionID}, nil
	}
	var wireTree wireMerkleTree
	if err := json.Unmarshal(data, &wireTree); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merkle tree: %w", err)
	}
	root := fromWireNode(wireTree.Root)
	keyRanges := collectKeyRanges(root)
	depth := wireTree.Depth
	if depth == 0 {
		depth = computeDepth(root)
	}
	return &MerkleTree{
		PartitionID: partitionID,
		Root:        root,
		KeyRanges:   keyRanges,
		Depth:       depth,
	}, nil
}

func toWireNode(n *MerkleNode) *wireMerkleNode {
	if n == nil {
		return nil
	}
	return &wireMerkleNode{
		Hash:     n.Hash,
		DataHash: n.DataHash,
		KeyRange: n.KeyRange,
		Left:     toWireNode(n.Left),
		Right:    toWireNode(n.Right),
	}
}

func fromWireNode(wn *wireMerkleNode) *MerkleNode {
	if wn == nil {
		return nil
	}
	return &MerkleNode{
		Hash:     wn.Hash,
		DataHash: wn.DataHash,
		KeyRange: wn.KeyRange,
		Left:     fromWireNode(wn.Left),
		Right:    fromWireNode(wn.Right),
	}
}

func collectKeyRanges(n *MerkleNode) []KeyRange {
	if n == nil {
		return []KeyRange{}
	}
	if n.Left == nil && n.Right == nil && n.KeyRange != nil {
		return []KeyRange{*n.KeyRange}
	}
	result := collectKeyRanges(n.Left)
	result = append(result, collectKeyRanges(n.Right)...)
	return result
}

func computeDepth(n *MerkleNode) int {
	if n == nil {
		return 0
	}
	left := computeDepth(n.Left)
	right := computeDepth(n.Right)
	if left > right {
		return left + 1
	}
	return right + 1
}

