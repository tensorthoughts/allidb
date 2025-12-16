package graph

import (
	"container/list"
	"sync"

	"github.com/tensorthoughts25/allidb/core/entity"
)

// EntityID is an alias for entity.EntityID
type EntityID = entity.EntityID

// GraphEdge is an alias for entity.GraphEdge
type GraphEdge = entity.GraphEdge

// lruNode represents a node in the LRU doubly-linked list.
type lruNode struct {
	entityID EntityID
	element  *list.Element
}

// GraphCache is an in-memory graph cache with adjacency list structure and LRU eviction.
type GraphCache struct {
	// Adjacency list: map from entity ID to list of outgoing edges
	outgoingEdges map[EntityID][]GraphEdge
	incomingEdges map[EntityID][]GraphEdge // For fast reverse lookups

	// LRU tracking
	lruList    *list.List                    // Doubly-linked list for LRU order
	lruMap     map[EntityID]*list.Element    // Map from entity ID to LRU list element
	maxSize    int                           // Maximum number of entities to cache
	currentSize int                          // Current number of entities

	// Synchronization
	mu sync.RWMutex // Protects all fields
}

// NewGraphCache creates a new graph cache with the specified maximum size.
// If maxSize is 0 or negative, a default of 10000 is used.
func NewGraphCache(maxSize int) *GraphCache {
	if maxSize <= 0 {
		maxSize = 10000 // Default cache size
	}

	return &GraphCache{
		outgoingEdges: make(map[EntityID][]GraphEdge),
		incomingEdges: make(map[EntityID][]GraphEdge),
		lruList:       list.New(),
		lruMap:        make(map[EntityID]*list.Element),
		maxSize:       maxSize,
		currentSize:   0,
	}
}

// AddEdge adds an edge to the cache.
// This method is thread-safe.
func (gc *GraphCache) AddEdge(edge GraphEdge) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	from := edge.FromEntity
	to := edge.ToEntity

	// Add to outgoing edges
	if _, exists := gc.outgoingEdges[from]; !exists {
		gc.outgoingEdges[from] = make([]GraphEdge, 0)
		gc.touchEntity(from) // Mark as recently used
	}
	gc.outgoingEdges[from] = append(gc.outgoingEdges[from], edge)

	// Add to incoming edges (for reverse traversal)
	if _, exists := gc.incomingEdges[to]; !exists {
		gc.incomingEdges[to] = make([]GraphEdge, 0)
		gc.touchEntity(to) // Mark as recently used
	}
	gc.incomingEdges[to] = append(gc.incomingEdges[to], edge)
}

// touchEntity marks an entity as recently used in the LRU cache.
// Must be called with lock held.
func (gc *GraphCache) touchEntity(entityID EntityID) {
	// If entity is already in cache, move to front
	if elem, exists := gc.lruMap[entityID]; exists {
		gc.lruList.MoveToFront(elem)
		return
	}

	// Add new entity
	if gc.currentSize >= gc.maxSize {
		gc.evictLRU()
	}

	elem := gc.lruList.PushFront(entityID)
	gc.lruMap[entityID] = elem
	gc.currentSize++
}

// evictLRU removes the least recently used entity from the cache.
// Must be called with lock held.
func (gc *GraphCache) evictLRU() {
	if gc.lruList.Len() == 0 {
		return
	}

	// Get the least recently used (back of list)
	back := gc.lruList.Back()
	if back == nil {
		return
	}

	entityID := back.Value.(EntityID)

	// Remove from maps
	delete(gc.outgoingEdges, entityID)
	delete(gc.incomingEdges, entityID)
	delete(gc.lruMap, entityID)

	// Remove from LRU list
	gc.lruList.Remove(back)
	gc.currentSize--
}

// GetOutgoingEdges returns all outgoing edges for the given entity.
// This method is thread-safe and marks the entity as recently used.
func (gc *GraphCache) GetOutgoingEdges(entityID EntityID) []GraphEdge {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Mark as recently used
	if _, exists := gc.outgoingEdges[entityID]; exists {
		gc.touchEntity(entityID)
	}

	edges, exists := gc.outgoingEdges[entityID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]GraphEdge, len(edges))
	copy(result, edges)
	return result
}

// GetIncomingEdges returns all incoming edges for the given entity.
// This method is thread-safe and marks the entity as recently used.
func (gc *GraphCache) GetIncomingEdges(entityID EntityID) []GraphEdge {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Mark as recently used
	if _, exists := gc.incomingEdges[entityID]; exists {
		gc.touchEntity(entityID)
	}

	edges, exists := gc.incomingEdges[entityID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]GraphEdge, len(edges))
	copy(result, edges)
	return result
}

// GetNeighbors returns all neighbor entity IDs (both incoming and outgoing).
// This method is thread-safe and marks the entity as recently used.
func (gc *GraphCache) GetNeighbors(entityID EntityID) []EntityID {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	neighborSet := make(map[EntityID]bool)

	// Add outgoing neighbors
	if outgoing, exists := gc.outgoingEdges[entityID]; exists {
		for _, edge := range outgoing {
			neighborSet[edge.ToEntity] = true
		}
	}

	// Add incoming neighbors
	if incoming, exists := gc.incomingEdges[entityID]; exists {
		for _, edge := range incoming {
			neighborSet[edge.FromEntity] = true
		}
	}

	// Mark as recently used if entity exists
	if _, exists := gc.outgoingEdges[entityID]; exists {
		gc.touchEntity(entityID)
	} else if _, exists := gc.incomingEdges[entityID]; exists {
		gc.touchEntity(entityID)
	}

	// Convert to slice
	neighbors := make([]EntityID, 0, len(neighborSet))
	for neighbor := range neighborSet {
		neighbors = append(neighbors, neighbor)
	}

	return neighbors
}

// Traverse performs a breadth-first traversal starting from the given entity.
// The visitor function is called for each visited entity. If the visitor returns false,
// traversal stops. Returns the number of entities visited.
// This method is thread-safe.
func (gc *GraphCache) Traverse(start EntityID, visitor func(EntityID, []GraphEdge) bool) int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	visited := make(map[EntityID]bool)
	queue := []EntityID{start}
	count := 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Get outgoing edges
		edges, exists := gc.outgoingEdges[current]
		if !exists {
			edges = []GraphEdge{}
		}

		// Call visitor
		if !visitor(current, edges) {
			break
		}
		count++

		// Add neighbors to queue
		for _, edge := range edges {
			if !visited[edge.ToEntity] {
				queue = append(queue, edge.ToEntity)
			}
		}
	}

	return count
}

// TraverseDepth performs a depth-first traversal starting from the given entity.
// The visitor function is called for each visited entity. If the visitor returns false,
// traversal stops. Returns the number of entities visited.
// This method is thread-safe.
func (gc *GraphCache) TraverseDepth(start EntityID, visitor func(EntityID, []GraphEdge) bool) int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	visited := make(map[EntityID]bool)
	stack := []EntityID{start}
	count := 0

	for len(stack) > 0 {
		// Pop from stack
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Get outgoing edges
		edges, exists := gc.outgoingEdges[current]
		if !exists {
			edges = []GraphEdge{}
		}

		// Call visitor
		if !visitor(current, edges) {
			break
		}
		count++

		// Add neighbors to stack (in reverse order for natural DFS)
		for i := len(edges) - 1; i >= 0; i-- {
			neighbor := edges[i].ToEntity
			if !visited[neighbor] {
				stack = append(stack, neighbor)
			}
		}
	}

	return count
}

// GetOutgoingNeighbors returns all entity IDs that the given entity points to.
// This method is thread-safe.
func (gc *GraphCache) GetOutgoingNeighbors(entityID EntityID) []EntityID {
	edges := gc.GetOutgoingEdges(entityID)
	neighbors := make([]EntityID, len(edges))
	for i, edge := range edges {
		neighbors[i] = edge.ToEntity
	}
	return neighbors
}

// GetIncomingNeighbors returns all entity IDs that point to the given entity.
// This method is thread-safe.
func (gc *GraphCache) GetIncomingNeighbors(entityID EntityID) []EntityID {
	edges := gc.GetIncomingEdges(entityID)
	neighbors := make([]EntityID, len(edges))
	for i, edge := range edges {
		neighbors[i] = edge.FromEntity
	}
	return neighbors
}

// Size returns the current number of entities in the cache.
func (gc *GraphCache) Size() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.currentSize
}

// MaxSize returns the maximum cache size.
func (gc *GraphCache) MaxSize() int {
	return gc.maxSize
}

// Clear removes all entries from the cache.
func (gc *GraphCache) Clear() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.outgoingEdges = make(map[EntityID][]GraphEdge)
	gc.incomingEdges = make(map[EntityID][]GraphEdge)
	gc.lruList = list.New()
	gc.lruMap = make(map[EntityID]*list.Element)
	gc.currentSize = 0
}

// RemoveEntity removes an entity and all its edges from the cache.
func (gc *GraphCache) RemoveEntity(entityID EntityID) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Remove outgoing edges
	if edges, exists := gc.outgoingEdges[entityID]; exists {
		// Remove reverse references (incoming edges in target entities)
		for _, edge := range edges {
			if incoming, exists := gc.incomingEdges[edge.ToEntity]; exists {
				newIncoming := make([]GraphEdge, 0, len(incoming))
				for _, e := range incoming {
					if e.FromEntity != entityID {
						newIncoming = append(newIncoming, e)
					}
				}
				gc.incomingEdges[edge.ToEntity] = newIncoming
			}
		}
		delete(gc.outgoingEdges, entityID)
	}

	// Remove incoming edges
	if edges, exists := gc.incomingEdges[entityID]; exists {
		// Remove reverse references (outgoing edges in source entities)
		for _, edge := range edges {
			if outgoing, exists := gc.outgoingEdges[edge.FromEntity]; exists {
				newOutgoing := make([]GraphEdge, 0, len(outgoing))
				for _, e := range outgoing {
					if e.ToEntity != entityID {
						newOutgoing = append(newOutgoing, e)
					}
				}
				gc.outgoingEdges[edge.FromEntity] = newOutgoing
			}
		}
		delete(gc.incomingEdges, entityID)
	}

	// Remove from LRU
	if elem, exists := gc.lruMap[entityID]; exists {
		gc.lruList.Remove(elem)
		delete(gc.lruMap, entityID)
		gc.currentSize--
	}
}

// HasEntity returns true if the entity exists in the cache.
func (gc *GraphCache) HasEntity(entityID EntityID) bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	_, exists := gc.outgoingEdges[entityID]
	return exists
}

// GetEdgeCount returns the total number of edges in the cache.
func (gc *GraphCache) GetEdgeCount() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	count := 0
	for _, edges := range gc.outgoingEdges {
		count += len(edges)
	}
	return count
}

