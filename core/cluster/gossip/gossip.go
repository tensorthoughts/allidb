package gossip

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// NodeID represents a node identifier.
type NodeID string

// NodeState represents the state of a node.
type NodeState int

const (
	// NodeStateAlive indicates the node is alive.
	NodeStateAlive NodeState = iota
	// NodeStateDead indicates the node is dead.
	NodeStateDead
	// NodeStateUnknown indicates the node state is unknown.
	NodeStateUnknown
)

// EndpointState represents the endpoint state of a node.
type EndpointState struct {
	mu sync.RWMutex

	NodeID      NodeID
	State       NodeState
	Heartbeat   int64     // Heartbeat version (monotonically increasing)
	Generation  int64     // Generation number (incremented on restart)
	LastUpdate  time.Time // Last time this state was updated
	ApplicationState map[string]interface{} // Application-specific state (e.g., SSTable versions)
}

// GossipDigest represents a digest of node state for efficient exchange.
type GossipDigest struct {
	NodeID    NodeID
	Generation int64
	Heartbeat  int64
}

// GossipDigestSyn represents a syn message (request for state).
type GossipDigestSyn struct {
	Digests []GossipDigest
}

// GossipDigestAck represents an ack message (response with state).
type GossipDigestAck struct {
	Digests []GossipDigest
	States  map[NodeID]*EndpointState
}

// GossipDigestAck2 represents an ack2 message (confirmation).
type GossipDigestAck2 struct {
	States map[NodeID]*EndpointState
}

// Node represents a remote node in the cluster.
type Node struct {
	NodeID  NodeID
	Address string
	Port    int
}

// Config holds configuration for the gossip protocol.
type Config struct {
	LocalNodeID      NodeID
	LocalAddress     string
	LocalPort        int
	SeedNodes        []string      // Seed nodes for initial cluster discovery (format: "host:port" or "node-id")
	GossipInterval   time.Duration // How often to gossip (default: 1s)
	FailureTimeout   time.Duration // Time before marking node as dead (default: 20s)
	CleanupInterval  time.Duration // How often to cleanup dead nodes (default: 60s)
	Fanout           int           // Number of nodes to gossip with per round (default: 3)
}

// DefaultConfig returns a default gossip configuration.
func DefaultConfig(localNodeID NodeID, localAddress string, localPort int) Config {
	return Config{
		LocalNodeID:     localNodeID,
		LocalAddress:    localAddress,
		LocalPort:       localPort,
		GossipInterval:  1 * time.Second,
		FailureTimeout:  20 * time.Second,
		CleanupInterval: 60 * time.Second,
		Fanout:          3,
	}
}

// Gossip implements the gossip protocol.
type Gossip struct {
	mu sync.RWMutex

	config Config

	// Local state
	localState *EndpointState
	generation int64

	// Remote node states
	endpointStates map[NodeID]*EndpointState

	// Known nodes in the cluster
	knownNodes map[NodeID]*Node

	// Transport layer (for sending/receiving messages)
	transport Transport

	// Control channels
	stopChan chan struct{}
	stopWg   sync.WaitGroup

	// Callbacks
	onNodeAlive func(NodeID)
	onNodeDead  func(NodeID)
}

// Transport is the interface for sending/receiving gossip messages.
type Transport interface {
	// SendSyn sends a syn message to a node.
	SendSyn(node *Node, syn *GossipDigestSyn) error
	// SendAck sends an ack message to a node.
	SendAck(node *Node, ack *GossipDigestAck) error
	// SendAck2 sends an ack2 message to a node.
	SendAck2(node *Node, ack2 *GossipDigestAck2) error
	// Receive receives messages from other nodes.
	Receive() <-chan GossipMessage
}

// GossipMessage represents a gossip message.
type GossipMessage struct {
	From    *Node
	Syn     *GossipDigestSyn
	Ack     *GossipDigestAck
	Ack2    *GossipDigestAck2
}

// NewGossip creates a new gossip instance.
func NewGossip(config Config, transport Transport) *Gossip {
	now := time.Now()
	generation := now.UnixNano()

	localState := &EndpointState{
		NodeID:          config.LocalNodeID,
		State:           NodeStateAlive,
		Heartbeat:       0,
		Generation:      generation,
		LastUpdate:      now,
		ApplicationState: make(map[string]interface{}),
	}

	g := &Gossip{
		config:         config,
		localState:     localState,
		generation:     generation,
		endpointStates: make(map[NodeID]*EndpointState),
		knownNodes:     make(map[NodeID]*Node),
		transport:      transport,
		stopChan:       make(chan struct{}),
	}

	// Seed known nodes from seed list (best-effort)
	for _, seed := range config.SeedNodes {
		host, portStr, err := net.SplitHostPort(seed)
		if err != nil {
			host = seed
			portStr = strconv.Itoa(config.LocalPort)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			port = config.LocalPort
		}
		// Skip if this is the local node
		if host == config.LocalAddress && port == config.LocalPort {
			continue
		}
		nodeID := NodeID(host)
		g.knownNodes[nodeID] = &Node{
			NodeID:  nodeID,
			Address: host,
			Port:    port,
		}
	}

	return g
}

// Start starts the gossip protocol.
func (g *Gossip) Start() error {
	// Add local node to known nodes
	g.mu.Lock()
	g.knownNodes[g.config.LocalNodeID] = &Node{
		NodeID:  g.config.LocalNodeID,
		Address: g.config.LocalAddress,
		Port:    g.config.LocalPort,
	}
	g.mu.Unlock()

	// Start gossip loop
	g.stopWg.Add(1)
	go g.gossipLoop()

	// Start message receiver
	g.stopWg.Add(1)
	go g.receiveLoop()

	// Start cleanup loop
	g.stopWg.Add(1)
	go g.cleanupLoop()

	return nil
}

// Stop stops the gossip protocol.
func (g *Gossip) Stop() {
	close(g.stopChan)
	g.stopWg.Wait()
}

// gossipLoop periodically gossips with random peers.
func (g *Gossip) gossipLoop() {
	defer g.stopWg.Done()

	ticker := time.NewTicker(g.config.GossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopChan:
			return
		case <-ticker.C:
			g.doGossip()
		}
	}
}

// doGossip performs a gossip round with random peers.
func (g *Gossip) doGossip() {
	g.mu.RLock()
	peers := g.selectRandomPeers(g.config.Fanout)
	g.mu.RUnlock()

	for _, peer := range peers {
		go g.gossipWithPeer(peer)
	}
}

// selectRandomPeers selects random peers to gossip with.
func (g *Gossip) selectRandomPeers(count int) []*Node {
	// Get all known nodes except local node
	candidates := make([]*Node, 0)
	for nodeID, node := range g.knownNodes {
		if nodeID != g.config.LocalNodeID {
			candidates = append(candidates, node)
		}
	}

	if len(candidates) == 0 {
		return []*Node{}
	}

	// Simple selection: take first N nodes
	// In production, use proper random selection
	selected := make([]*Node, 0, count)
	for i := 0; i < count && i < len(candidates); i++ {
		selected = append(selected, candidates[i])
	}

	return selected
}

// gossipWithPeer gossips with a specific peer.
func (g *Gossip) gossipWithPeer(peer *Node) {
	// Build digest of our knowledge
	digest := g.buildDigest()

	// Send SYN
	syn := &GossipDigestSyn{
		Digests: digest,
	}

	if err := g.transport.SendSyn(peer, syn); err != nil {
		// Log error but continue
		return
	}

	// In a real implementation, we'd wait for ACK and send ACK2
	// For now, this is a simplified version
}

// buildDigest builds a digest of known node states.
func (g *Gossip) buildDigest() []GossipDigest {
	g.mu.RLock()
	defer g.mu.RUnlock()

	digest := make([]GossipDigest, 0, len(g.endpointStates)+1)

	// Add local state
	digest = append(digest, GossipDigest{
		NodeID:    g.localState.NodeID,
		Generation: g.localState.Generation,
		Heartbeat:  g.localState.Heartbeat,
	})

	// Add remote states
	for nodeID, state := range g.endpointStates {
		state.mu.RLock()
		digest = append(digest, GossipDigest{
			NodeID:    nodeID,
			Generation: state.Generation,
			Heartbeat:  state.Heartbeat,
		})
		state.mu.RUnlock()
	}

	return digest
}

// receiveLoop receives and processes gossip messages.
func (g *Gossip) receiveLoop() {
	defer g.stopWg.Done()

	for {
		select {
		case <-g.stopChan:
			return
		case msg := <-g.transport.Receive():
			g.handleMessage(&msg)
		}
	}
}

// handleMessage handles an incoming gossip message.
func (g *Gossip) handleMessage(msg *GossipMessage) {
	if msg.Syn != nil {
		g.handleSyn(msg.From, msg.Syn)
	} else if msg.Ack != nil {
		g.handleAck(msg.From, msg.Ack)
	} else if msg.Ack2 != nil {
		g.handleAck2(msg.From, msg.Ack2)
	}
}

// handleSyn handles a SYN message.
func (g *Gossip) handleSyn(from *Node, syn *GossipDigestSyn) {
	// Build digest of what we have that they might need
	requestedStates := make(map[NodeID]*EndpointState)
	ourDigest := make([]GossipDigest, 0)

	g.mu.RLock()
	// Add local state to our digest
	ourDigest = append(ourDigest, GossipDigest{
		NodeID:    g.localState.NodeID,
		Generation: g.localState.Generation,
		Heartbeat:  g.localState.Heartbeat,
	})

	// Compare digests and determine what to send
	for _, theirDigest := range syn.Digests {
		ourState, exists := g.endpointStates[theirDigest.NodeID]
		if !exists {
			// They know about a node we don't - request it
			continue
		}

		ourState.mu.RLock()
		ourGen := ourState.Generation
		ourHeartbeat := ourState.Heartbeat
		ourState.mu.RUnlock()

		// If their version is newer, request update
		if theirDigest.Generation > ourGen ||
			(theirDigest.Generation == ourGen && theirDigest.Heartbeat > ourHeartbeat) {
			// Request update (they'll send it in ACK)
			continue
		}

		// If our version is newer, send it
		if ourGen > theirDigest.Generation ||
			(ourGen == theirDigest.Generation && ourHeartbeat > theirDigest.Heartbeat) {
			requestedStates[theirDigest.NodeID] = ourState
		}
	}

	// Add local state if they need it
	needLocalState := true
	for _, d := range syn.Digests {
		if d.NodeID == g.localState.NodeID {
			if d.Generation < g.localState.Generation ||
				(d.Generation == g.localState.Generation && d.Heartbeat < g.localState.Heartbeat) {
				requestedStates[g.localState.NodeID] = g.localState
			}
			needLocalState = false
			break
		}
	}
	if needLocalState {
		requestedStates[g.localState.NodeID] = g.localState
	}
	g.mu.RUnlock()

	// Send ACK with requested states
	ack := &GossipDigestAck{
		Digests: ourDigest,
		States:  requestedStates,
	}

	g.transport.SendAck(from, ack)
}

// handleAck handles an ACK message.
func (g *Gossip) handleAck(from *Node, ack *GossipDigestAck) {
	// Update our state with received states
	g.mu.Lock()
	defer g.mu.Unlock()

	for nodeID, state := range ack.States {
		g.updateEndpointState(nodeID, state)
	}

	// Send ACK2 with any states they might need
	ack2States := make(map[NodeID]*EndpointState)
	for _, digest := range ack.Digests {
		ourState, exists := g.endpointStates[digest.NodeID]
		if exists {
			ourState.mu.RLock()
			ourGen := ourState.Generation
			ourHeartbeat := ourState.Heartbeat
			ourState.mu.RUnlock()

			if ourGen > digest.Generation ||
				(ourGen == digest.Generation && ourHeartbeat > digest.Heartbeat) {
				ack2States[digest.NodeID] = ourState
			}
		}
	}

	ack2 := &GossipDigestAck2{
		States: ack2States,
	}

	g.transport.SendAck2(from, ack2)
}

// handleAck2 handles an ACK2 message.
func (g *Gossip) handleAck2(from *Node, ack2 *GossipDigestAck2) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for nodeID, state := range ack2.States {
		g.updateEndpointState(nodeID, state)
	}
}

// updateEndpointState updates the endpoint state for a node.
func (g *Gossip) updateEndpointState(nodeID NodeID, newState *EndpointState) {
	existing, exists := g.endpointStates[nodeID]
	if !exists {
		// Create new state
		existing = &EndpointState{
			NodeID:          nodeID,
			ApplicationState: make(map[string]interface{}),
		}
		g.endpointStates[nodeID] = existing
	}

	existing.mu.Lock()
	defer existing.mu.Unlock()

	newState.mu.RLock()
	newGen := newState.Generation
	newHeartbeat := newState.Heartbeat
	newState.mu.RUnlock()

	// Update if new state is newer
	if newGen > existing.Generation ||
		(newGen == existing.Generation && newHeartbeat > existing.Heartbeat) {
		existing.Generation = newGen
		existing.Heartbeat = newHeartbeat
		existing.State = NodeStateAlive
		existing.LastUpdate = time.Now()

		// Update application state
		newState.mu.RLock()
		for k, v := range newState.ApplicationState {
			existing.ApplicationState[k] = v
		}
		newState.mu.RUnlock()

		// Notify if node became alive
		if existing.State == NodeStateAlive && g.onNodeAlive != nil {
			go g.onNodeAlive(nodeID)
		}
	}
}

// cleanupLoop periodically checks for dead nodes.
func (g *Gossip) cleanupLoop() {
	defer g.stopWg.Done()

	ticker := time.NewTicker(g.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopChan:
			return
		case <-ticker.C:
			g.checkDeadNodes()
		}
	}
}

// checkDeadNodes marks nodes as dead if they haven't been updated recently.
func (g *Gossip) checkDeadNodes() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for nodeID, state := range g.endpointStates {
		state.mu.RLock()
		lastUpdate := state.LastUpdate
		state.mu.RUnlock()

		if now.Sub(lastUpdate) > g.config.FailureTimeout {
			state.mu.Lock()
			if state.State == NodeStateAlive {
				state.State = NodeStateDead
				if g.onNodeDead != nil {
					go g.onNodeDead(nodeID)
				}
			}
			state.mu.Unlock()
		}
	}
}

// AddNode adds a known node to the cluster.
func (g *Gossip) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.knownNodes[node.NodeID] = node
}

// GetNodeAddress returns address:port for a node if known and address is set.
func (g *Gossip) GetNodeAddress(nodeID NodeID) (string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.knownNodes[nodeID]
	if !ok || node.Address == "" {
		return "", false
	}
	return fmt.Sprintf("%s:%d", node.Address, node.Port), true
}

// RemoveNode removes a node from known nodes.
func (g *Gossip) RemoveNode(nodeID NodeID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.knownNodes, nodeID)
	delete(g.endpointStates, nodeID)
}

// IncrementHeartbeat increments the local node's heartbeat.
func (g *Gossip) IncrementHeartbeat() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.localState.Heartbeat++
	g.localState.LastUpdate = time.Now()
}

// UpdateApplicationState updates the local node's application state.
func (g *Gossip) UpdateApplicationState(key string, value interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.localState.ApplicationState[key] = value
	g.localState.LastUpdate = time.Now()
}

// GetSSTableVersion gets the SSTable version for a node.
func (g *Gossip) GetSSTableVersion(nodeID NodeID) (int64, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	state, exists := g.endpointStates[nodeID]
	if !exists {
		return 0, false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	version, ok := state.ApplicationState["sstable_version"].(int64)
	return version, ok
}

// SetSSTableVersion sets the local node's SSTable version.
func (g *Gossip) SetSSTableVersion(version int64) {
	g.UpdateApplicationState("sstable_version", version)
}

// GetNodeState gets the state of a node.
func (g *Gossip) GetNodeState(nodeID NodeID) (NodeState, bool) {
	if nodeID == g.config.LocalNodeID {
		return NodeStateAlive, true
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	state, exists := g.endpointStates[nodeID]
	if !exists {
		return NodeStateUnknown, false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.State, true
}

// GetAliveNodes returns all alive nodes.
func (g *Gossip) GetAliveNodes() []NodeID {
	g.mu.RLock()
	defer g.mu.RUnlock()

	alive := make([]NodeID, 0)
	alive = append(alive, g.config.LocalNodeID) // Local node is always alive

	for nodeID, state := range g.endpointStates {
		state.mu.RLock()
		if state.State == NodeStateAlive {
			alive = append(alive, nodeID)
		}
		state.mu.RUnlock()
	}

	return alive
}

// SetOnNodeAlive sets a callback for when a node becomes alive.
func (g *Gossip) SetOnNodeAlive(callback func(NodeID)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onNodeAlive = callback
}

// SetOnNodeDead sets a callback for when a node becomes dead.
func (g *Gossip) SetOnNodeDead(callback func(NodeID)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onNodeDead = callback
}

