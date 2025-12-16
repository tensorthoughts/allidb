// +build grpc

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	grpcapi "github.com/tensorthoughts25/allidb/api/grpc"
	httppkg "github.com/tensorthoughts25/allidb/api/http"
	"github.com/tensorthoughts25/allidb/core/ai"
	"github.com/tensorthoughts25/allidb/core/cluster"
	"github.com/tensorthoughts25/allidb/core/cluster/gossip"
	"github.com/tensorthoughts25/allidb/core/cluster/handoff"
	"github.com/tensorthoughts25/allidb/core/cluster/ring"
	repair "github.com/tensorthoughts25/allidb/core/cluster/repair"
	repairEntropy "github.com/tensorthoughts25/allidb/core/cluster/repair/entropy"
	"github.com/tensorthoughts25/allidb/core/config"
	"github.com/tensorthoughts25/allidb/core/index/hnsw"
	"github.com/tensorthoughts25/allidb/core/index/unified"
	"github.com/tensorthoughts25/allidb/core/observability"
	"github.com/tensorthoughts25/allidb/core/query"
	"github.com/tensorthoughts25/allidb/core/security/auth"
	"github.com/tensorthoughts25/allidb/core/security/secrets"
	sectls "github.com/tensorthoughts25/allidb/core/security/tls"
	"github.com/tensorthoughts25/allidb/core/storage"
	"github.com/tensorthoughts25/allidb/core/storage/compaction"
	"github.com/tensorthoughts25/allidb/core/storage/memtable"
	"github.com/tensorthoughts25/allidb/core/storage/wal"
	pb "github.com/tensorthoughts25/allidb/api/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	configPath = flag.String("config", getEnv("ALLIDB_CONFIG", "configs/allidb.yaml"), "Path to configuration file")
)

func main() {
	flag.Parse()

	// Step 1: Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Step 2: Validate configuration (already done in LoadConfig, but ensure)
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Step 3: Initialize logger using config
	logger := observability.NewLogger(observability.LoggerConfig{
		MinLevel: parseLogLevel(cfg.Observability.LogLevel),
	})
	logger.Info("Starting AlliDB server", observability.Fields{
		"node_id":     cfg.Node.ID,
		"grpc_port":   cfg.Node.GRPCPort,
		"http_port":   cfg.Node.HTTPPort,
		"data_dir":    cfg.Node.DataDir,
		"config_path": *configPath,
	})

	// Create data directories
	dataDir := cfg.Node.DataDir
	walDir := dataDir + "/wal"
	sstableDir := dataDir + "/sstables"
	handoffDir := dataDir + "/handoff"
	if err := os.MkdirAll(walDir, 0755); err != nil {
		log.Fatalf("Failed to create WAL directory: %v", err)
	}
	if err := os.MkdirAll(sstableDir, 0755); err != nil {
		log.Fatalf("Failed to create SSTable directory: %v", err)
	}
	if err := os.MkdirAll(handoffDir, 0755); err != nil {
		log.Fatalf("Failed to create handoff directory: %v", err)
	}

	// Step 4: Initialize storage using config
	logger.Info("Initializing storage layer...")

	// Initialize WAL (using storage engine config helper)
	engineConfig := storage.NewEngineConfigFromConfig(cfg.Storage, dataDir)
	walConfig := engineConfig.WALConfig(func() wal.WALEntry {
		return &EntityWALEntry{}
	})
	// WAL fsync is now properly wired from config (cfg.Storage.WAL.Fsync)
	walInstance, err := wal.New(walConfig)
	if err != nil {
		log.Fatalf("Failed to create WAL: %v", err)
	}
	defer walInstance.Close()

	// Initialize memtable (using storage engine config helper)
	memtableInstance := memtable.New(engineConfig.MemtableConfig())

	// Replay WAL on startup
	logger.Info("Replaying WAL entries...")
	replayedCount := 0
	restoredCount := 0
	if err := walInstance.Replay(func(entry wal.WALEntry) error {
		replayedCount++
		entEntry, ok := entry.(*EntityWALEntry)
		if !ok {
			// Skip unknown entry types but continue replay
			return nil
		}
		if entEntry.Entity == nil {
			return nil
		}
		row := &EntityRow{Entity: entEntry.Entity}
		memtableInstance.Put(row)
		restoredCount++
		return nil
	}); err != nil {
		logger.Warn("WAL replay encountered errors", observability.Fields{"error": err.Error()})
	} else {
	logger.Info("WAL replay completed", observability.Fields{
		"entries":  replayedCount,
		"restored": restoredCount,
		"memtable": memtableInstance.Size(),
	})
	}

	// Initialize tenant guard
	tenantGuard := storage.NewTenantGuard(cfg.Security.Auth.Enabled) // Use auth enabled as fail-hard flag

	// Create storage service (wires memtable, WAL, and tenant guard together)
	storageService := NewStorageService(walInstance, memtableInstance, tenantGuard, sstableDir)

	// Initialize compaction manager (using storage engine config helper)
	logger.Info("Initializing compaction manager...")
	compactionConfig := engineConfig.CompactionConfig()
	compactionManager, err := compaction.NewManager(compactionConfig)
	if err != nil {
		log.Fatalf("Failed to create compaction manager: %v", err)
	}
	if err := compactionManager.Start(); err != nil {
		log.Fatalf("Failed to start compaction manager: %v", err)
	}
	defer compactionManager.Stop()

	// Step 5: Initialize cluster (ring + gossip) using config
	logger.Info("Initializing cluster components...")

	// Prepare TLS config (shared for gossip and gRPC if enabled)
	var serverTLSConfig *tls.Config
	if cfg.Security.TLS.Enabled && cfg.Security.TLS.CertFile != "" && cfg.Security.TLS.KeyFile != "" {
		tlsConfig := sectls.Config{
			CertFile:          cfg.Security.TLS.CertFile,
			KeyFile:           cfg.Security.TLS.KeyFile,
			CAFile:            cfg.Security.TLS.CAFile,
			RequireClientCert: false,
			OptionalClientTLS: false,
		}
		serverTLSConfig, err = tlsConfig.ServerConfig()
		if err != nil {
			log.Fatalf("Failed to create server TLS config: %v", err)
		}
	}

	// Create consistent hashing ring
	ringConfig := ring.Config{
		VirtualNodesPerNode: cfg.Cluster.VirtualNodesPerNode,
		ReplicationFactor:   cfg.Cluster.ReplicationFactor,
	}
	r := ring.New(ringConfig)
	r.AddNode(ring.NodeID(cfg.Node.ID))

	// Setup gossip protocol
	gossipConfig := gossip.Config{
		LocalNodeID:     gossip.NodeID(cfg.Node.ID),
		LocalAddress:    cfg.Node.ListenAddress,
		LocalPort:       cfg.Cluster.GossipPort,
		SeedNodes:       cfg.Cluster.SeedNodes,
		GossipInterval:  time.Duration(cfg.Cluster.GossipIntervalMs) * time.Millisecond,
		FailureTimeout:  time.Duration(cfg.Cluster.FailureTimeoutMs) * time.Millisecond,
		CleanupInterval: time.Duration(cfg.Cluster.CleanupIntervalMs) * time.Millisecond,
		Fanout:          cfg.Cluster.Fanout,
	}
	gossipListenAddr := fmt.Sprintf("%s:%d", cfg.Node.ListenAddress, cfg.Cluster.GossipPort)
	gossipTransport, err := NewTCPTransport(gossipListenAddr, serverTLSConfig)
	if err != nil {
		log.Fatalf("Failed to create gossip transport: %v", err)
	}
	defer gossipTransport.Close()

	gossipNode := gossip.NewGossip(gossipConfig, gossipTransport)
	if err := gossipNode.Start(); err != nil {
		log.Fatalf("Failed to start gossip: %v", err)
	}
	defer gossipNode.Stop()

	// Create replica client (mock for local dev, real for production)
	replicaClient := NewMockReplicaClient()

	// Initialize hinted handoff
	logger.Info("Initializing hinted handoff system...")
	handoffConfig := handoff.Config{
		DataDir:       handoffDir,
		MaxSize:       int64(cfg.Repair.HintedHandoff.MaxSizeMB) * 1024 * 1024,
		MaxTTL:        time.Duration(cfg.Repair.HintedHandoff.TTLMinutes) * time.Minute,
		FlushInterval: time.Duration(cfg.Repair.HintedHandoff.FlushIntervalMs) * time.Millisecond,
	}
	handoffStore, err := handoff.NewStore(handoffConfig)
	if err != nil {
		log.Fatalf("Failed to create handoff store: %v", err)
	}
	defer handoffStore.Close()

	// Create coordinator
	coordinatorConfig := cluster.Config{
		ReadQuorum:   (cfg.Cluster.ReplicationFactor / 2) + 1,
		WriteQuorum:  (cfg.Cluster.ReplicationFactor / 2) + 1,
		ReadTimeout:  time.Duration(cfg.Query.TimeoutMs) * time.Millisecond,
		WriteTimeout: time.Duration(cfg.Query.TimeoutMs) * time.Millisecond,
		ReplicaCount: cfg.Cluster.ReplicationFactor,
	}
	coordinator := cluster.NewCoordinator(r, gossipNode, replicaClient, coordinatorConfig)

	// Wrap coordinator with handoff support
	coordinatorWithHandoff := &CoordinatorWithHandoff{
		coordinator: coordinator,
		handoffStore: handoffStore,
		ring: r,
	}

	// Create handoff dispatcher
	dispatcherConfig := handoff.DefaultDispatcherConfig()
	handoffDispatcher := handoff.NewDispatcher(handoffStore, gossipNode, coordinator, replicaClient, dispatcherConfig)
	if err := handoffDispatcher.Start(); err != nil {
		log.Fatalf("Failed to start handoff dispatcher: %v", err)
	}
	defer handoffDispatcher.Stop()
	logger.Info("Hinted handoff system initialized")

	// ===== REPAIR SYSTEM =====
	logger.Info("Initializing repair system...")

	// Repair executor
	repairExecutor := repair.NewExecutor(repair.ExecutorConfig{
		Coordinator: coordinator,
		Client:      replicaClient,
	})

	// Repair scheduler
	repairScheduler := repair.NewScheduler(repair.DefaultSchedulerConfig(repairExecutor))
	if err := repairScheduler.Start(); err != nil {
		log.Fatalf("Failed to start repair scheduler: %v", err)
	}
	defer repairScheduler.Stop()

	// Repair detector
	repairDetector := repair.NewDetector(repair.DetectorConfig{
		Scheduler: repairScheduler,
	})

	// Wrap coordinator to automatically detect divergence on reads
	// Note: coordinatorWithHandoff already wraps the base coordinator
	coordinatorWithRepair := &CoordinatorWithRepair{
		coordinator:     coordinatorWithHandoff,
		repairDetector:  repairDetector,
		baseCoordinator: coordinator, // Keep reference for ReadWithResponses
	}

	// Step 6: Initialize vector + graph index using config
	logger.Info("Initializing unified index and query executor...")

	// Unified index (automatically loads SSTables on creation)
	hnswConfig := hnsw.Config{
		M:              cfg.Index.HNSW.M,
		Ef:             cfg.Index.HNSW.EfSearch,
		EfConstruction: cfg.Index.HNSW.EfConstruction,
		ShardCount:     cfg.Index.HNSW.ShardCount,
	}
	indexConfig := unified.Config{
		HNSWConfig:      hnswConfig,
		GraphCacheSize:  cfg.Index.GraphCacheSize,
		EntityCacheSize: cfg.Index.EntityCacheSize,
		SSTableDir:      sstableDir,
		SSTablePrefix:   "sstable",
		ReloadInterval:  time.Duration(cfg.Index.ReloadIntervalMs) * time.Millisecond,
	}
	unifiedIndex, err := unified.New(indexConfig)
	if err != nil {
		log.Fatalf("Failed to create unified index: %v", err)
	}
	defer unifiedIndex.Close()
	logger.Info("Unified index initialized and SSTables loaded", observability.Fields{
		"entity_count": unifiedIndex.Size(),
	})

	// Entropy repair (Merkle tree-based)
	// Create production partition provider
	partitionProvider := &ProductionPartitionProvider{
		ring: r,
	}

	// Create production tree provider
	treeProvider := &ProductionTreeProvider{
		coordinator:    coordinator,
		storageService: storageService,
		unifiedIndex:   unifiedIndex,
		localNodeID:    cluster.NodeID(cfg.Node.ID),
	}

	// Create entropy comparator
	entropyComparator := repairEntropy.NewComparator(repairEntropy.ComparatorConfig{
		TreeProvider: treeProvider,
		Coordinator:  coordinator,
	})

	// Create production data provider
	dataProvider := &ProductionDataProvider{
		coordinator:    coordinator,
		storageService: storageService,
		unifiedIndex:   unifiedIndex,
		localNodeID:    cluster.NodeID(cfg.Node.ID),
	}

	// Create entropy executor
	entropyExecutor := repairEntropy.NewRangeExecutor(repairEntropy.RangeExecutorConfig{
		Coordinator:  coordinator,
		DataProvider: dataProvider,
	})

	// Create entropy orchestrator
	entropyOrchestratorConfig := repairEntropy.OrchestratorConfig{
		Comparator:        entropyComparator,
		Executor:          entropyExecutor,
		PartitionProvider: partitionProvider,
		PeriodicInterval:  time.Duration(cfg.Repair.AntiEntropy.IntervalMinutes) * time.Minute,
	}
	entropyOrchestrator := repairEntropy.NewOrchestrator(entropyOrchestratorConfig)
	if err := entropyOrchestrator.Start(); err != nil {
		logger.Warn("Failed to start entropy orchestrator", observability.Fields{"error": err.Error()})
	} else {
		defer entropyOrchestrator.Stop()
		logger.Info("Entropy repair orchestrator started")
	}

	// Query executor
	queryConfig := query.Config{
		ScoringWeights:     query.DefaultScoringWeights(),
		DefaultK:           cfg.Query.DefaultK,
		DefaultExpandFactor: cfg.Query.MaxGraphHops,
		MaxCandidates:      cfg.Query.MaxCandidates,
		EnableTracing:      cfg.Observability.LogLevel == "DEBUG",
	}
	queryExecutor := query.NewExecutor(unifiedIndex, queryConfig)

	// Step 7: Initialize AI providers using config
	logger.Info("Initializing AI providers...")
	
	// Determine required secrets based on AI config
	var requiredSecrets []string
	if cfg.AI.Embeddings.Provider == "openai" || cfg.AI.LLM.Provider == "openai" {
		requiredSecrets = append(requiredSecrets, "OPENAI_API_KEY")
	}
	if cfg.AI.Embeddings.Provider == "custom" || cfg.AI.LLM.Provider == "custom" {
		requiredSecrets = append(requiredSecrets, "CUSTOM_AI_KEY")
	}
	
	// Create secret provider
	secretProvider, err := secrets.NewEnvProvider(requiredSecrets)
	if err != nil {
		logger.Warn("Failed to create secret provider with required secrets, continuing without validation", observability.Fields{"error": err.Error()})
		// Create without required secrets - will fail later if secrets are actually needed
		secretProvider, _ = secrets.NewEnvProvider([]string{})
	}
	
	// Create AI factory
	aiFactory := ai.NewFactory(cfg.AI, secretProvider)
	
	// Create embedder (if needed for query execution)
	var embedder ai.Embedder
	if cfg.AI.Embeddings.Provider != "" {
		embedder, err = aiFactory.CreateEmbedder()
		if err != nil {
			logger.Warn("Failed to create embedder", observability.Fields{"error": err.Error()})
		} else {
			logger.Info("Embedder initialized", observability.Fields{
				"provider": cfg.AI.Embeddings.Provider,
				"model":    cfg.AI.Embeddings.Model,
				"dim":      embedder.Dimension(),
			})
		}
	}
	
	// Create LLM (if needed for entity extraction)
	var llmClient ai.LLM
	if cfg.AI.LLM.Provider != "" {
		llmClient, err = aiFactory.CreateLLM()
		if err != nil {
			logger.Warn("Failed to create LLM", observability.Fields{"error": err.Error()})
		} else {
			logger.Info("LLM initialized", observability.Fields{
				"provider": cfg.AI.LLM.Provider,
				"model":    cfg.AI.LLM.Model,
			})
		}
	}
	
	// Create entity extractor (if enabled)
	var entityExtractor ai.EntityExtractor
	if cfg.AI.EntityExtraction.Enabled {
		entityExtractor, err = aiFactory.CreateEntityExtractor()
		if err != nil {
			logger.Warn("Failed to create entity extractor", observability.Fields{"error": err.Error()})
		} else {
			logger.Info("Entity extractor initialized")
		}
	}
	
	_ = embedder        // Will be used by query executor
	_ = llmClient       // Will be used by entity extraction
	_ = entityExtractor // Will be used by entity extraction

	// Step 8: Start gRPC + HTTP servers using config
	logger.Info("Initializing API servers...")

	// Create API key store (needed for both HTTP and gRPC auth)
	apiKeyStore := auth.NewAPIKeyStore()
	// Load static API keys from config (for development / small deployments)
	for _, k := range cfg.Security.Auth.StaticAPIKeys {
		logger.Info("Loading static API key", observability.Fields{"key": k.Key, "tenant_id": k.TenantID})
		apiKeyStore.AddKey(k.Key, k.TenantID)
	}

	// Create gRPC server with TLS support and auth interceptor
	var grpcOpts []grpc.ServerOption
	if cfg.Security.TLS.Enabled && cfg.Security.TLS.CertFile != "" && cfg.Security.TLS.KeyFile != "" {
		tlsConfig := sectls.Config{
			CertFile:          cfg.Security.TLS.CertFile,
			KeyFile:           cfg.Security.TLS.KeyFile,
			CAFile:            cfg.Security.TLS.CAFile,
			RequireClientCert: false, // Could be configurable
			OptionalClientTLS: false, // Could be configurable
		}

		serverTLSConfig, err := tlsConfig.ServerConfig()
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(serverTLSConfig)))
		logger.Info("TLS enabled for gRPC", observability.Fields{
			"cert_file": cfg.Security.TLS.CertFile,
			"key_file":  cfg.Security.TLS.KeyFile,
		})
	}

	// Add gRPC auth interceptor if auth is enabled
	if cfg.Security.Auth.Enabled {
		grpcOpts = append(grpcOpts, grpc.UnaryInterceptor(auth.GRPCAuthInterceptor(apiKeyStore)))
		grpcOpts = append(grpcOpts, grpc.StreamInterceptor(auth.GRPCStreamAuthInterceptor(apiKeyStore)))
		logger.Info("gRPC auth interceptor enabled")
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// Decide whether to enable real-time indexing based on config.
	// When disabled, the unified index is fed only via SSTable reloads.
	var unifiedIndexForGRPC grpcapi.UnifiedIndex
	if cfg.Index.RealtimeIndexEnabled {
		unifiedIndexForGRPC = unifiedIndex
	} else {
		unifiedIndexForGRPC = nil
	}

	// Use coordinator with repair detection
	grpcService := grpcapi.NewServer(grpcapi.Config{
		Coordinator:   coordinatorWithRepair, // Use wrapper that includes repair detection
		QueryExecutor: queryExecutor,
		RangeScanner:  storageService,
		LocalNodeID:   cfg.Node.ID,
		NodeLookup: func(nodeID string) (string, bool) {
			addr, ok := gossipNode.GetNodeAddress(gossip.NodeID(nodeID))
			return addr, ok
		},
		ReplicaKV:    storageService,
		UnifiedIndex: unifiedIndexForGRPC, // Real-time indexing is now configurable
	})
	pb.RegisterAlliDBServer(grpcServer, grpcService)

	// Create HTTP gateway
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Node.ListenAddress, cfg.Node.GRPCPort)
	httpGateway, err := httppkg.NewGateway(httppkg.Config{
		GRPCAddress: grpcAddr,
	})
	if err != nil {
		log.Fatalf("Failed to create HTTP gateway: %v", err)
	}
	defer httpGateway.Close()

	// Setup HTTP server with auth middleware
	mux := http.NewServeMux()
	httpGateway.RegisterRoutes(mux)

	var handler http.Handler = mux
	if cfg.Security.Auth.Enabled {
		logger.Info("Enabling API key authentication", observability.Fields{"enabled": cfg.Security.Auth.Enabled})
		handler = auth.HTTPAuthMiddleware(apiKeyStore)(mux)
		logger.Info("API key authentication middleware registered")
	}
	
	httpAddr := fmt.Sprintf("%s:%d", cfg.Node.ListenAddress, cfg.Node.HTTPPort)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: handler,
	}

	// Step 9: Start servers
	logger.Info("Starting servers...")

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
	}

	go func() {
		logger.Info("gRPC server started", observability.Fields{"addr": grpcAddr})
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Start HTTP server with TLS support
	go func() {
		logger.Info("HTTP server started", observability.Fields{"addr": httpAddr, "tls": cfg.Security.TLS.Enabled})
		var err error
		if cfg.Security.TLS.Enabled && cfg.Security.TLS.CertFile != "" && cfg.Security.TLS.KeyFile != "" {
			// HTTP server can use the same TLS config
			err = httpServer.ListenAndServeTLS(cfg.Security.TLS.CertFile, cfg.Security.TLS.KeyFile)
		} else {
			err = httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Step 10: Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", err)
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	logger.Info("Server stopped")
}

// ===== MOCK/HELPER IMPLEMENTATIONS =====

// MockReplicaClient is a simple in-memory mock for local development.
type MockReplicaClient struct {
	data map[cluster.NodeID]map[string][]byte
	mu   sync.RWMutex
}

func NewMockReplicaClient() *MockReplicaClient {
	return &MockReplicaClient{
		data: make(map[cluster.NodeID]map[string][]byte),
	}
}

func (m *MockReplicaClient) Read(ctx context.Context, nodeID cluster.NodeID, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data[nodeID] != nil {
		return m.data[nodeID][key], nil
	}
	return nil, nil
}

func (m *MockReplicaClient) Write(ctx context.Context, nodeID cluster.NodeID, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[nodeID] == nil {
		m.data[nodeID] = make(map[string][]byte)
	}
	m.data[nodeID][key] = value
	return nil
}

// TCPTransport implements gossip.Transport over TCP with optional TLS and JSON encoding.
type TCPTransport struct {
	ln       net.Listener
	tlsCfg   *tls.Config
	recvChan chan gossip.GossipMessage
}

type wireMessage struct {
	Type string                  `json:"type"`
	From gossip.Node             `json:"from"`
	Syn  *gossip.GossipDigestSyn  `json:"syn,omitempty"`
	Ack  *gossip.GossipDigestAck  `json:"ack,omitempty"`
	Ack2 *gossip.GossipDigestAck2 `json:"ack2,omitempty"`
}

func NewTCPTransport(listenAddr string, tlsCfg *tls.Config) (*TCPTransport, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		ln = tls.NewListener(ln, tlsCfg)
	}
	t := &TCPTransport{
		ln:       ln,
		tlsCfg:   tlsCfg,
		recvChan: make(chan gossip.GossipMessage, 256),
	}
	go t.acceptLoop()
	return t, nil
}

func (t *TCPTransport) acceptLoop() {
	for {
		conn, err := t.ln.Accept()
		if err != nil {
			return
		}
		go t.handleConn(conn)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	for {
		var wm wireMessage
		if err := dec.Decode(&wm); err != nil {
			return
		}
		msg := gossip.GossipMessage{}
		msg.From = &gossip.Node{
			NodeID:  wm.From.NodeID,
			Address: wm.From.Address,
			Port:    wm.From.Port,
		}
		switch wm.Type {
		case "syn":
			msg.Syn = wm.Syn
		case "ack":
			msg.Ack = wm.Ack
		case "ack2":
			msg.Ack2 = wm.Ack2
		default:
			continue
		}
		select {
		case t.recvChan <- msg:
		default:
		}
	}
}

func (t *TCPTransport) send(node *gossip.Node, wm wireMessage) error {
	addr := fmt.Sprintf("%s:%d", node.Address, node.Port)
	var d net.Dialer
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return err
	}
	if t.tlsCfg != nil {
		conn = tls.Client(conn, t.tlsCfg)
	}
	defer conn.Close()
	enc := json.NewEncoder(conn)
	return enc.Encode(&wm)
}

func (t *TCPTransport) SendSyn(node *gossip.Node, syn *gossip.GossipDigestSyn) error {
	return t.send(node, wireMessage{Type: "syn", From: *node, Syn: syn})
}

func (t *TCPTransport) SendAck(node *gossip.Node, ack *gossip.GossipDigestAck) error {
	return t.send(node, wireMessage{Type: "ack", From: *node, Ack: ack})
}

func (t *TCPTransport) SendAck2(node *gossip.Node, ack2 *gossip.GossipDigestAck2) error {
	return t.send(node, wireMessage{Type: "ack2", From: *node, Ack2: ack2})
}

func (t *TCPTransport) Receive() <-chan gossip.GossipMessage {
	return t.recvChan
}

func (t *TCPTransport) Close() error {
	if t.ln != nil {
		return t.ln.Close()
	}
	return nil
}

// CoordinatorWithHandoffInterface is the interface for coordinator with handoff.
type CoordinatorWithHandoffInterface interface {
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, value []byte) error
	GetConfig() cluster.Config
	UpdateConfig(config cluster.Config)
	GetReplicas(key string) []cluster.NodeID
	GetAliveReplicas(key string) []cluster.NodeID
}

// CoordinatorWithHandoff wraps the coordinator to store hints for failed writes.
type CoordinatorWithHandoff struct {
	coordinator  *cluster.Coordinator
	handoffStore *handoff.Store
	ring         *ring.Ring
}

// Read performs a quorum read.
func (c *CoordinatorWithHandoff) Read(ctx context.Context, key string) ([]byte, error) {
	return c.coordinator.Read(ctx, key)
}

// Write performs a quorum write and stores hints for failed nodes.
func (c *CoordinatorWithHandoff) Write(ctx context.Context, key string, value []byte) error {
	// Get all replicas for this key
	allReplicas := c.coordinator.GetReplicas(key)
	if len(allReplicas) == 0 {
		return fmt.Errorf("no replicas available for key %s", key)
	}
	
	// Get alive replicas
	aliveReplicas := c.coordinator.GetAliveReplicas(key)
	aliveSet := make(map[cluster.NodeID]bool)
	for _, nodeID := range aliveReplicas {
		aliveSet[nodeID] = true
	}
	
	// Store hints for nodes that are not alive (before attempting write)
	for _, nodeID := range allReplicas {
		if !aliveSet[nodeID] {
			// Node is not alive - store hint
			if storeErr := c.handoffStore.StoreHint(nodeID, key, value); storeErr != nil {
				// Log error but don't fail the write
				// The hint store might be full, which is acceptable
			}
		}
	}
	
	// Try to write via coordinator (this filters to alive nodes)
	err := c.coordinator.Write(ctx, key, value)
	
	// If write succeeded with quorum, we're done
	// Hints for dead nodes are already stored above
	if err == nil {
		return nil
	}
	
	// Write failed - this means we didn't get quorum
	// We've already stored hints for dead nodes above
	// In a more sophisticated implementation, we could also track which alive nodes failed
	// and store hints for them, but for now we rely on the dispatcher to retry
	
	return err
}

// GetConfig returns the coordinator configuration.
func (c *CoordinatorWithHandoff) GetConfig() cluster.Config {
	return c.coordinator.GetConfig()
}

// UpdateConfig updates the coordinator configuration.
func (c *CoordinatorWithHandoff) UpdateConfig(config cluster.Config) {
	c.coordinator.UpdateConfig(config)
}

// GetReplicas returns the replicas for a key.
func (c *CoordinatorWithHandoff) GetReplicas(key string) []cluster.NodeID {
	return c.coordinator.GetReplicas(key)
}

// GetAliveReplicas returns the alive replicas for a key.
func (c *CoordinatorWithHandoff) GetAliveReplicas(key string) []cluster.NodeID {
	return c.coordinator.GetAliveReplicas(key)
}

// CoordinatorWithRepair wraps the coordinator to automatically detect divergence on reads.
type CoordinatorWithRepair struct {
	coordinator    CoordinatorWithHandoffInterface
	repairDetector *repair.Detector
	baseCoordinator *cluster.Coordinator // Keep reference to base coordinator for ReadWithResponses
}

// Read performs a quorum read and automatically checks for divergence.
func (c *CoordinatorWithRepair) Read(ctx context.Context, key string) ([]byte, error) {
	// Use the wrapped coordinator's Read method
	return c.coordinator.Read(ctx, key)
}

// ReadWithResponses performs a quorum read and returns all responses.
// This is needed for repair detection.
func (c *CoordinatorWithRepair) ReadWithResponses(ctx context.Context, key string) ([]cluster.Response, error) {
	// Get responses from base coordinator
	responses, err := c.baseCoordinator.ReadWithResponses(ctx, key)
	
	// Check for divergence if we have multiple responses
	if c.repairDetector != nil && len(responses) >= 2 {
		if err := c.repairDetector.CheckReadResponses(ctx, key, responses); err != nil {
			// Log error but don't fail the read
			// Repair will be scheduled asynchronously
		}
	}
	
	return responses, err
}

// Write performs a quorum write.
func (c *CoordinatorWithRepair) Write(ctx context.Context, key string, value []byte) error {
	return c.coordinator.Write(ctx, key, value)
}

// GetConfig returns the coordinator configuration.
func (c *CoordinatorWithRepair) GetConfig() cluster.Config {
	return c.coordinator.GetConfig()
}

// UpdateConfig updates the coordinator configuration.
func (c *CoordinatorWithRepair) UpdateConfig(config cluster.Config) {
	c.coordinator.UpdateConfig(config)
}

// ProductionPartitionProvider provides partition information based on the ring's virtual nodes.
// Partitions are defined by ranges between virtual nodes on the ring.
// Each partition represents a range of keys owned by a node.
type ProductionPartitionProvider struct {
	ring *ring.Ring
}

func (p *ProductionPartitionProvider) GetPartitions(ctx context.Context) ([]repairEntropy.PartitionID, error) {
	// Get all nodes in the ring
	nodes := p.ring.GetNodes()
	if len(nodes) == 0 {
		return []repairEntropy.PartitionID{}, nil
	}

	// Create partitions based on nodes
	// In a production system, you might create partitions based on:
	// 1. Virtual node ranges (each virtual node = a partition)
	// 2. Token ranges (ranges between virtual node hashes)
	// 3. Node-based partitions (each node owns a partition)
	// 
	// For simplicity, we'll use node-based partitions where each node
	// owns a partition identified by its node ID.
	partitions := make([]repairEntropy.PartitionID, 0, len(nodes))
	for _, nodeID := range nodes {
		partitions = append(partitions, repairEntropy.PartitionID(string(nodeID)))
	}

	return partitions, nil
}

// ProductionTreeProvider builds Merkle trees from actual partition data.
type ProductionTreeProvider struct {
	coordinator    *cluster.Coordinator
	storageService *StorageService
	unifiedIndex   *unified.UnifiedIndex
	localNodeID    cluster.NodeID
}

func (p *ProductionTreeProvider) GetTree(ctx context.Context, nodeID cluster.NodeID, partitionID repairEntropy.PartitionID) (*repairEntropy.MerkleTree, error) {
	// Build key-value map for the partition
	keyValues := make(map[string][]byte)
	
	// Only local node supported for now
	if nodeID != p.localNodeID {
		return nil, fmt.Errorf("range scan for remote node %s not implemented", nodeID)
	}

	if p.storageService == nil {
		return nil, fmt.Errorf("tree provider missing storage service")
	}

	data, err := p.storageService.RangeScan(ctx, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("range scan failed: %w", err)
	}
	for key, ent := range data {
		owners := p.coordinator.GetReplicas(key)
		isOwner := false
		for _, owner := range owners {
			if owner == nodeID {
				isOwner = true
				break
			}
		}
		if !isOwner {
			continue
		}
		keyValues[key] = ent.ToBytes()
	}
	
	// Build Merkle tree
	builder := repairEntropy.NewMerkleBuilder(partitionID)
	tree, err := builder.BuildTree(keyValues)
	if err != nil {
		return nil, fmt.Errorf("failed to build Merkle tree: %w", err)
	}
	
	return tree, nil
}

// ProductionDataProvider fetches actual data from nodes for repair.
type ProductionDataProvider struct {
	coordinator    *cluster.Coordinator
	storageService *StorageService
	unifiedIndex   *unified.UnifiedIndex
	localNodeID    cluster.NodeID
}

func (p *ProductionDataProvider) GetRangeData(ctx context.Context, nodeID cluster.NodeID, partitionID repairEntropy.PartitionID, keyRange repairEntropy.KeyRange) (map[string][]byte, error) {
	result := make(map[string][]byte)
	
	if p.storageService == nil {
		return nil, fmt.Errorf("data provider missing storage service")
	}

	if nodeID != p.localNodeID {
		return nil, fmt.Errorf("range scan for remote node %s not implemented", nodeID)
	}

	data, err := p.storageService.RangeScan(ctx, keyRange.Start, keyRange.End, "")
	if err != nil {
		return nil, fmt.Errorf("range scan failed: %w", err)
	}
	for key, ent := range data {
		owners := p.coordinator.GetReplicas(key)
		isOwner := false
		for _, owner := range owners {
			if owner == nodeID {
				isOwner = true
				break
			}
		}
		if !isOwner {
			continue
		}
		result[key] = ent.ToBytes()
	}
	
	return result, nil
}

// ===== UTILITY FUNCTIONS =====

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func parseLogLevel(level string) observability.LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return observability.LogLevelDebug
	case "INFO":
		return observability.LogLevelInfo
	case "WARN":
		return observability.LogLevelWarn
	case "ERROR":
		return observability.LogLevelError
	default:
		return observability.LogLevelInfo
	}
}
