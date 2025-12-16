// +build grpc

package grpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/api/grpc/pb"
	"github.com/tensorthoughts25/allidb/core/cluster"
	"github.com/tensorthoughts25/allidb/core/cluster/gossip"
	"github.com/tensorthoughts25/allidb/core/cluster/ring"
	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/query"
	"github.com/tensorthoughts25/allidb/core/security/tenant"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockReplicaClient is a mock replica client for testing coordinator.
type MockReplicaClient struct {
	mu        sync.RWMutex
	writeData map[string][]byte
	writeErr  error
	readData  map[string][]byte
	readErr   error
	writeKeys []string
}

func NewMockReplicaClient() *MockReplicaClient {
	return &MockReplicaClient{
		writeData: make(map[string][]byte),
		readData:  make(map[string][]byte),
		writeKeys: make([]string, 0),
	}
}

func (m *MockReplicaClient) Read(ctx context.Context, nodeID cluster.NodeID, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	return m.readData[key], nil
}

func (m *MockReplicaClient) Write(ctx context.Context, nodeID cluster.NodeID, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.writeData[key] = value
	m.writeKeys = append(m.writeKeys, key)
	return nil
}

func (m *MockReplicaClient) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

func (m *MockReplicaClient) GetWriteKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.writeKeys))
	copy(result, m.writeKeys)
	return result
}

func (m *MockReplicaClient) GetWriteData(key string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writeData[key]
}

// MockGossip is a mock gossip for testing coordinator.
type MockGossip struct {
	aliveNodes []gossip.NodeID
}

func NewMockGossip() *MockGossip {
	return &MockGossip{
		aliveNodes: []gossip.NodeID{"node1", "node2", "node3"},
	}
}

func (m *MockGossip) GetAliveNodes() []gossip.NodeID {
	return m.aliveNodes
}

func (m *MockGossip) GetNodeState(nodeID gossip.NodeID) (gossip.NodeState, bool) {
	return gossip.NodeStateUnknown, false
}

// QueryExecutorInterface defines the interface for query execution.
type QueryExecutorInterface interface {
	Query(ctx context.Context, queryVector []float32, k int) ([]query.QueryResult, error)
	QueryWithOptions(ctx context.Context, queryVector []float32, k int, expandFactor int) ([]query.QueryResult, error)
}

// MockQueryExecutor is a mock query executor for testing.
type MockQueryExecutor struct {
	mu            sync.RWMutex
	queryCalls    []QueryCall
	queryResults  []query.QueryResult
	queryError    error
}

type QueryCall struct {
	QueryVector  []float32
	K            int
	ExpandFactor int
}

func NewMockQueryExecutor() *MockQueryExecutor {
	return &MockQueryExecutor{
		queryCalls:   make([]QueryCall, 0),
		queryResults: make([]query.QueryResult, 0),
	}
}

func (m *MockQueryExecutor) Query(ctx context.Context, queryVector []float32, k int) ([]query.QueryResult, error) {
	return m.QueryWithOptions(ctx, queryVector, k, 2)
}

func (m *MockQueryExecutor) QueryWithOptions(ctx context.Context, queryVector []float32, k int, expandFactor int) ([]query.QueryResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls = append(m.queryCalls, QueryCall{
		QueryVector:  queryVector,
		K:            k,
		ExpandFactor: expandFactor,
	})
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResults, nil
}

func (m *MockQueryExecutor) SetQueryResults(results []query.QueryResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryResults = results
}

func (m *MockQueryExecutor) SetQueryError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryError = err
}

func (m *MockQueryExecutor) GetQueryCalls() []QueryCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]QueryCall, len(m.queryCalls))
	copy(result, m.queryCalls)
	return result
}

func setupTestServer() (*Server, *cluster.Coordinator, *MockQueryExecutor, *MockReplicaClient) {
	// Create a real coordinator with mocked dependencies
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	coordinator := cluster.NewCoordinator(ring, mockGossip, mockClient, cluster.DefaultConfig())

	mockExecutor := NewMockQueryExecutor()

	server := NewServer(Config{
		Coordinator:   coordinator,
		QueryExecutor: mockExecutor,
	})
	return server, coordinator, mockExecutor, mockClient
}

func contextWithTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenant(ctx, tenantID)
}

func createTestEntity(tenantID, entityID string) *pb.Entity {
	return &pb.Entity{
		EntityId:   entityID,
		TenantId:   tenantID,
		EntityType: "test",
		Embeddings: []*pb.VectorEmbedding{
			{
				Version:        1,
				Vector:         []float32{1.0, 0.0, 0.0},
				TimestampNanos: time.Now().UnixNano(),
			},
		},
		Importance:     0.5,
		CreatedAtNanos: time.Now().UnixNano(),
		UpdatedAtNanos: time.Now().UnixNano(),
	}
}

func TestNewServer(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	coordinator := cluster.NewCoordinator(ring, mockGossip, mockClient, cluster.DefaultConfig())
	mockExecutor := NewMockQueryExecutor()

	server := NewServer(Config{
		Coordinator:   coordinator,
		QueryExecutor: mockExecutor,
	})

	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.coordinator == nil {
		t.Error("Coordinator not set")
	}
	if server.queryExecutor == nil {
		t.Error("QueryExecutor not set")
	}
}

func TestServer_PutEntity_Success(t *testing.T) {
	server, _, _, mockClient := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")
	entity := createTestEntity("tenant1", "entity1")

	req := &pb.PutEntityRequest{
		Entity:           entity,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	resp, err := server.PutEntity(ctx, req)

	if err != nil {
		t.Fatalf("PutEntity failed: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success, got failure")
	}
	if resp.EntityId != "entity1" {
		t.Errorf("Expected entity_id 'entity1', got '%s'", resp.EntityId)
	}

	// Verify coordinator was called
	writeKeys := mockClient.GetWriteKeys()
	if len(writeKeys) == 0 {
		t.Fatal("Expected at least one write call")
	}
	// Check that the key contains tenant and entity
	found := false
	for _, key := range writeKeys {
		if key == "tenant1:entity1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected key 'tenant1:entity1' in write keys, got %v", writeKeys)
	}
}

func TestServer_PutEntity_NoTenant(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := context.Background() // No tenant in context
	entity := createTestEntity("tenant1", "entity1")

	req := &pb.PutEntityRequest{
		Entity:           entity,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.PutEntity(ctx, req)

	if err == nil {
		t.Fatal("Expected error for missing tenant")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("Expected Unauthenticated, got %v", st.Code())
	}
}

func TestServer_PutEntity_NoEntity(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	req := &pb.PutEntityRequest{
		Entity:           nil,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.PutEntity(ctx, req)

	if err == nil {
		t.Fatal("Expected error for nil entity")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got %v", st.Code())
	}
}

func TestServer_PutEntity_EmptyEntityID(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")
	entity := createTestEntity("tenant1", "")

	req := &pb.PutEntityRequest{
		Entity:           entity,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.PutEntity(ctx, req)

	if err == nil {
		t.Fatal("Expected error for empty entity_id")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got %v", st.Code())
	}
}

func TestServer_PutEntity_TenantMismatch(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")
	entity := createTestEntity("tenant2", "entity1") // Different tenant

	req := &pb.PutEntityRequest{
		Entity:           entity,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.PutEntity(ctx, req)

	if err == nil {
		t.Fatal("Expected error for tenant mismatch")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("Expected PermissionDenied, got %v", st.Code())
	}
}

func TestServer_PutEntity_CoordinatorError(t *testing.T) {
	server, _, _, mockClient := setupTestServer()
	mockClient.SetWriteError(errors.New("coordinator error"))
	ctx := contextWithTenant(context.Background(), "tenant1")
	entity := createTestEntity("tenant1", "entity1")

	req := &pb.PutEntityRequest{
		Entity:           entity,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.PutEntity(ctx, req)

	if err == nil {
		t.Fatal("Expected error from coordinator")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal, got %v", st.Code())
	}
}

func TestServer_PutEntity_ConsistencyLevels(t *testing.T) {
	levels := []pb.ConsistencyLevel{
		pb.ConsistencyLevel_CONSISTENCY_ONE,
		pb.ConsistencyLevel_CONSISTENCY_QUORUM,
		pb.ConsistencyLevel_CONSISTENCY_ALL,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			server, _, _, _ := setupTestServer()
			ctx := contextWithTenant(context.Background(), "tenant1")
			entity := createTestEntity("tenant1", "entity1")

			req := &pb.PutEntityRequest{
				Entity:           entity,
				ConsistencyLevel: level,
			}

			// Just verify the call succeeds with different consistency levels
			// The actual config update is an implementation detail that gets restored
			_, err := server.PutEntity(ctx, req)
			if err != nil {
				t.Fatalf("PutEntity failed: %v", err)
			}
		})
	}
}

func TestServer_BatchPutEntities_Success(t *testing.T) {
	server, _, _, mockClient := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	entities := []*pb.Entity{
		createTestEntity("tenant1", "entity1"),
		createTestEntity("tenant1", "entity2"),
		createTestEntity("tenant1", "entity3"),
	}

	req := &pb.BatchPutEntitiesRequest{
		Entities:         entities,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	resp, err := server.BatchPutEntities(ctx, req)

	if err != nil {
		t.Fatalf("BatchPutEntities failed: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success, got failure")
	}
	if resp.SuccessCount != 3 {
		t.Errorf("Expected SuccessCount 3, got %d", resp.SuccessCount)
	}
	if resp.FailureCount != 0 {
		t.Errorf("Expected FailureCount 0, got %d", resp.FailureCount)
	}
	if len(resp.EntityIds) != 3 {
		t.Errorf("Expected 3 entity IDs, got %d", len(resp.EntityIds))
	}

	// Verify all entities were written
	writeKeys := mockClient.GetWriteKeys()
	if len(writeKeys) < 3 {
		t.Fatalf("Expected at least 3 write calls, got %d", len(writeKeys))
	}
}

func TestServer_BatchPutEntities_Empty(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	req := &pb.BatchPutEntitiesRequest{
		Entities:         []*pb.Entity{},
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.BatchPutEntities(ctx, req)

	if err == nil {
		t.Fatal("Expected error for empty entities")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got %v", st.Code())
	}
}

func TestServer_BatchPutEntities_PartialFailure(t *testing.T) {
	server, _, _, mockClient := setupTestServer()
	mockClient.SetWriteError(errors.New("write error"))
	ctx := contextWithTenant(context.Background(), "tenant1")

	entities := []*pb.Entity{
		createTestEntity("tenant1", "entity1"),
		createTestEntity("tenant1", "entity2"),
	}

	req := &pb.BatchPutEntitiesRequest{
		Entities:         entities,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	resp, err := server.BatchPutEntities(ctx, req)

	if err != nil {
		t.Fatalf("BatchPutEntities should not return error for partial failure: %v", err)
	}
	if resp.Success {
		t.Error("Expected failure due to write errors")
	}
	if resp.SuccessCount != 0 {
		t.Errorf("Expected SuccessCount 0, got %d", resp.SuccessCount)
	}
	if resp.FailureCount != 2 {
		t.Errorf("Expected FailureCount 2, got %d", resp.FailureCount)
	}
}

func TestServer_BatchPutEntities_TenantMismatch(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	entities := []*pb.Entity{
		createTestEntity("tenant1", "entity1"),
		createTestEntity("tenant2", "entity2"), // Different tenant
	}

	req := &pb.BatchPutEntitiesRequest{
		Entities:         entities,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	resp, err := server.BatchPutEntities(ctx, req)

	if err != nil {
		t.Fatalf("BatchPutEntities should not return error: %v", err)
	}
	if resp.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", resp.SuccessCount)
	}
	if resp.FailureCount != 1 {
		t.Errorf("Expected FailureCount 1, got %d", resp.FailureCount)
	}
}

func TestServer_Query_Success(t *testing.T) {
	server, _, mockExecutor, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	// Create test entity
	testEntity := entity.NewUnifiedEntity("entity1", "tenant1", "test")
	testEntity.AddEmbedding([]float32{1.0, 0.0, 0.0})

	// Set up mock executor results
	mockExecutor.SetQueryResults([]query.QueryResult{
		{
			EntityID: "entity1",
			Score:    0.95,
			Entity:   testEntity,
		},
	})

	req := &pb.QueryRequest{
		QueryVector:      []float32{1.0, 0.0, 0.0},
		K:                10,
		ExpandFactor:     2,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
		TraceId:          "trace123",
	}

	resp, err := server.Query(ctx, req)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].EntityId != "entity1" {
		t.Errorf("Expected entity_id 'entity1', got '%s'", resp.Results[0].EntityId)
	}
	if resp.Results[0].Score != 0.95 {
		t.Errorf("Expected score 0.95, got %f", resp.Results[0].Score)
	}
	if resp.TraceId != "trace123" {
		t.Errorf("Expected trace_id 'trace123', got '%s'", resp.TraceId)
	}

	// Verify executor was called
	queryCalls := mockExecutor.GetQueryCalls()
	if len(queryCalls) != 1 {
		t.Fatalf("Expected 1 query call, got %d", len(queryCalls))
	}
	if queryCalls[0].K != 10 {
		t.Errorf("Expected K 10, got %d", queryCalls[0].K)
	}
	if queryCalls[0].ExpandFactor != 2 {
		t.Errorf("Expected ExpandFactor 2, got %d", queryCalls[0].ExpandFactor)
	}
}

func TestServer_Query_EmptyVector(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	req := &pb.QueryRequest{
		QueryVector:      []float32{},
		K:                10,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.Query(ctx, req)

	if err == nil {
		t.Fatal("Expected error for empty query vector")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got %v", st.Code())
	}
}

func TestServer_Query_DefaultK(t *testing.T) {
	server, _, mockExecutor, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	req := &pb.QueryRequest{
		QueryVector:      []float32{1.0, 0.0, 0.0},
		K:                0, // Should default to 10
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.Query(ctx, req)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	queryCalls := mockExecutor.GetQueryCalls()
	if len(queryCalls) != 1 {
		t.Fatalf("Expected 1 query call, got %d", len(queryCalls))
	}
	if queryCalls[0].K != 10 {
		t.Errorf("Expected default K 10, got %d", queryCalls[0].K)
	}
}

func TestServer_Query_TenantFiltering(t *testing.T) {
	server, _, mockExecutor, _ := setupTestServer()
	ctx := contextWithTenant(context.Background(), "tenant1")

	// Create entities with different tenants
	entity1 := entity.NewUnifiedEntity("entity1", "tenant1", "test")
	entity1.AddEmbedding([]float32{1.0, 0.0, 0.0})

	entity2 := entity.NewUnifiedEntity("entity2", "tenant2", "test") // Different tenant
	entity2.AddEmbedding([]float32{0.0, 1.0, 0.0})

	mockExecutor.SetQueryResults([]query.QueryResult{
		{EntityID: "entity1", Score: 0.95, Entity: entity1},
		{EntityID: "entity2", Score: 0.90, Entity: entity2},
	})

	req := &pb.QueryRequest{
		QueryVector:      []float32{1.0, 0.0, 0.0},
		K:                10,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	resp, err := server.Query(ctx, req)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	// Should only return entity1 (tenant1), not entity2 (tenant2)
	if len(resp.Results) != 1 {
		t.Fatalf("Expected 1 result after tenant filtering, got %d", len(resp.Results))
	}
	if resp.Results[0].EntityId != "entity1" {
		t.Errorf("Expected entity_id 'entity1', got '%s'", resp.Results[0].EntityId)
	}
}

func TestServer_Query_ExecutorError(t *testing.T) {
	server, _, mockExecutor, _ := setupTestServer()
	mockExecutor.SetQueryError(errors.New("executor error"))
	ctx := contextWithTenant(context.Background(), "tenant1")

	req := &pb.QueryRequest{
		QueryVector:      []float32{1.0, 0.0, 0.0},
		K:                10,
		ConsistencyLevel: pb.ConsistencyLevel_CONSISTENCY_QUORUM,
	}

	_, err := server.Query(ctx, req)

	if err == nil {
		t.Fatal("Expected error from executor")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal, got %v", st.Code())
	}
}

func TestServer_HealthCheck_Healthy(t *testing.T) {
	server, _, _, _ := setupTestServer()
	ctx := context.Background()

	req := &pb.HealthCheckRequest{}
	resp, err := server.HealthCheck(ctx, req)

	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	if !resp.Healthy {
		t.Error("Expected healthy, got unhealthy")
	}
	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", resp.Status)
	}
	if resp.TimestampNanos == 0 {
		t.Error("Expected timestamp to be set")
	}
}

func TestServer_HealthCheck_Unhealthy(t *testing.T) {
	// Server with nil coordinator
	ring := ring.New(ring.DefaultConfig())
	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	coordinator := cluster.NewCoordinator(ring, mockGossip, mockClient, cluster.DefaultConfig())
	
	server := NewServer(Config{
		Coordinator:   coordinator,
		QueryExecutor: nil, // nil executor should make it unhealthy
	})
	ctx := context.Background()

	req := &pb.HealthCheckRequest{}
	resp, err := server.HealthCheck(ctx, req)

	if err != nil {
		t.Fatalf("HealthCheck should not fail: %v", err)
	}
	if resp.Healthy {
		t.Error("Expected unhealthy, got healthy")
	}
	if resp.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%s'", resp.Status)
	}
}

func TestServer_ProtoToEntity(t *testing.T) {
	server, _, _, _ := setupTestServer()
	tenantID := "tenant1"

	protoEntity := &pb.Entity{
		EntityId:   "entity1",
		TenantId:   tenantID,
		EntityType: "test",
		Embeddings: []*pb.VectorEmbedding{
			{
				Version:        1,
				Vector:         []float32{1.0, 0.0, 0.0},
				TimestampNanos: time.Now().UnixNano(),
			},
		},
		IncomingEdges: []*pb.GraphEdge{
			{
				FromEntity:     "entity2",
				ToEntity:       "entity1",
				Relation:       "related_to",
				Weight:         0.8,
				TimestampNanos: time.Now().UnixNano(),
			},
		},
		OutgoingEdges: []*pb.GraphEdge{
			{
				FromEntity:     "entity1",
				ToEntity:       "entity3",
				Relation:       "related_to",
				Weight:         0.7,
				TimestampNanos: time.Now().UnixNano(),
			},
		},
		Chunks: []*pb.TextChunk{
			{
				ChunkId:  "chunk1",
				Text:     "test text",
				Metadata: map[string]string{"key": "value"},
			},
		},
		Importance: 0.5,
	}

	ent := server.protoToEntity(protoEntity, tenantID)

	if ent.GetEntityID() != "entity1" {
		t.Errorf("Expected entity_id 'entity1', got '%s'", ent.GetEntityID())
	}
	if ent.GetTenantID() != tenantID {
		t.Errorf("Expected tenant_id '%s', got '%s'", tenantID, ent.GetTenantID())
	}
	if len(ent.GetEmbeddings()) != 1 {
		t.Errorf("Expected 1 embedding, got %d", len(ent.GetEmbeddings()))
	}
	if len(ent.GetIncomingEdges()) != 1 {
		t.Errorf("Expected 1 incoming edge, got %d", len(ent.GetIncomingEdges()))
	}
	if len(ent.GetOutgoingEdges()) != 1 {
		t.Errorf("Expected 1 outgoing edge, got %d", len(ent.GetOutgoingEdges()))
	}
	if len(ent.GetChunks()) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(ent.GetChunks()))
	}
	if ent.GetImportance() != 0.5 {
		t.Errorf("Expected importance 0.5, got %f", ent.GetImportance())
	}
}

func TestServer_EntityToProto(t *testing.T) {
	server, _, _, _ := setupTestServer()

	ent := entity.NewUnifiedEntity("entity1", "tenant1", "test")
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})
	ent.AddIncomingEdge(entity.GraphEdge{
		FromEntity: "entity2",
		ToEntity:   "entity1",
		Relation:   "related_to",
		Weight:     0.8,
		Timestamp:  time.Now(),
	})
	ent.AddChunk(entity.TextChunk{
		ChunkID:  "chunk1",
		Text:     "test text",
		Metadata: map[string]string{"key": "value"},
	})
	ent.SetImportance(0.5)

	protoEntity := server.entityToProto(ent)

	if protoEntity.EntityId != "entity1" {
		t.Errorf("Expected entity_id 'entity1', got '%s'", protoEntity.EntityId)
	}
	if protoEntity.TenantId != "tenant1" {
		t.Errorf("Expected tenant_id 'tenant1', got '%s'", protoEntity.TenantId)
	}
	if len(protoEntity.Embeddings) != 1 {
		t.Errorf("Expected 1 embedding, got %d", len(protoEntity.Embeddings))
	}
	if len(protoEntity.IncomingEdges) != 1 {
		t.Errorf("Expected 1 incoming edge, got %d", len(protoEntity.IncomingEdges))
	}
	if len(protoEntity.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(protoEntity.Chunks))
	}
	if protoEntity.Importance != 0.5 {
		t.Errorf("Expected importance 0.5, got %f", protoEntity.Importance)
	}
}

