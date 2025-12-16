// +build grpc

package grpc

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/tensorthoughts25/allidb/api/grpc/pb"
	"github.com/tensorthoughts25/allidb/core/cluster"
	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/query"
	"github.com/tensorthoughts25/allidb/core/security/tenant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QueryExecutor defines the interface for query execution.
type QueryExecutor interface {
	Query(ctx context.Context, queryVector []float32, k int) ([]query.QueryResult, error)
	QueryWithOptions(ctx context.Context, queryVector []float32, k int, expandFactor int) ([]query.QueryResult, error)
}

// CoordinatorService defines the interface for the coordinator.
type CoordinatorService interface {
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, value []byte) error
	GetConfig() cluster.Config
	UpdateConfig(config cluster.Config)
}

// ReplicaKV defines replication read/write.
type ReplicaKV interface {
	ReplicaRead(ctx context.Context, key, tenantID string) ([]byte, error)
	ReplicaWrite(ctx context.Context, key, tenantID string, value []byte) error
}

// UnifiedIndex defines the interface for adding entities to the index.
// This interface is used to enable immediate indexing for demo purposes.
type UnifiedIndex interface {
	AddEntity(ent *entity.UnifiedEntity) error // AddEntity adds an entity to the index immediately
}

// Server implements the AlliDB gRPC service.
type Server struct {
	pb.UnimplementedAlliDBServer

	coordinator   CoordinatorService
	queryExecutor QueryExecutor
	rangeScanner  RangeScanner
	localNodeID   string
	nodeLookup    func(nodeID string) (string, bool) // nodeID -> address
	replicaKV     ReplicaKV
	unifiedIndex  UnifiedIndex // Optional: for immediate indexing (demo mode)
}

// Config holds configuration for the gRPC server.
type Config struct {
	Coordinator   CoordinatorService
	QueryExecutor QueryExecutor
	RangeScanner  RangeScanner
	LocalNodeID   string
	NodeLookup    func(nodeID string) (string, bool)
	ReplicaKV     ReplicaKV
	UnifiedIndex  UnifiedIndex // Optional: for immediate indexing (demo mode)
}

// NewServer creates a new gRPC server.
func NewServer(config Config) *Server {
	return &Server{
		coordinator:   config.Coordinator,
		queryExecutor: config.QueryExecutor,
		rangeScanner:  config.RangeScanner,
		localNodeID:   config.LocalNodeID,
		nodeLookup:    config.NodeLookup,
		replicaKV:     config.ReplicaKV,
		unifiedIndex:  config.UnifiedIndex,
	}
}

// RangeScanner defines range scan behavior.
type RangeScanner interface {
	RangeScan(ctx context.Context, start, end, tenantID string) (map[string]*entity.UnifiedEntity, error)
}

// PutEntity stores a single entity.
func (s *Server) PutEntity(ctx context.Context, req *pb.PutEntityRequest) (*pb.PutEntityResponse, error) {
	// Extract tenant ID from context (set by auth middleware)
	tenantID, err := tenant.RequireTenant(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "tenant required: %v", err)
	}

	// Validate request
	if req.Entity == nil {
		return nil, status.Errorf(codes.InvalidArgument, "entity is required")
	}
	if req.Entity.EntityId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "entity_id is required")
	}

	// Validate tenant ID matches entity tenant ID
	if req.Entity.TenantId != tenantID {
		return nil, status.Errorf(codes.PermissionDenied, "tenant ID mismatch")
	}

	// Convert consistency level to coordinator config
	coordinatorConfig := s.getCoordinatorConfig(req.ConsistencyLevel)

	// Convert entity to UnifiedEntity
	ent := s.protoToEntity(req.Entity, tenantID)

	// Serialize entity to bytes
	entityBytes := ent.ToBytes()

	// Use coordinator to write with specified consistency level
	key := fmt.Sprintf("%s:%s", tenantID, req.Entity.EntityId)

	// Temporarily update coordinator config
	oldConfig := s.coordinator.GetConfig()
	s.coordinator.UpdateConfig(coordinatorConfig)
	defer s.coordinator.UpdateConfig(oldConfig)

	err = s.coordinator.Write(ctx, key, entityBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write entity: %v", err)
	}

	// Add to unified index immediately (for demo/real-time indexing)
	// In production, entities are indexed via SSTable hot reload
	if s.unifiedIndex != nil {
		_ = s.unifiedIndex.AddEntity(ent) // Ignore errors for now
	}

	// Return response
	return &pb.PutEntityResponse{
		Success:  true,
		EntityId: req.Entity.EntityId,
	}, nil
}

// BatchPutEntities stores multiple entities in a single request.
func (s *Server) BatchPutEntities(ctx context.Context, req *pb.BatchPutEntitiesRequest) (*pb.BatchPutEntitiesResponse, error) {
	// Extract tenant ID from context
	tenantID, err := tenant.RequireTenant(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "tenant required: %v", err)
	}

	entities := req.Entities
	if len(entities) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "at least one entity is required")
	}

	// Convert consistency level
	coordinatorConfig := s.getCoordinatorConfig(req.ConsistencyLevel)

	// Process entities
	successCount := 0
	failureCount := 0
	entityIDs := make([]string, 0, len(entities))
	var lastErr error

	for _, protoEntity := range entities {
		// Validate tenant ID
		if protoEntity.TenantId != tenantID {
			failureCount++
			lastErr = fmt.Errorf("tenant ID mismatch for entity %s", protoEntity.EntityId)
			continue
		}

		// Convert and write entity
		ent := s.protoToEntity(protoEntity, tenantID)
		entityBytes := ent.ToBytes()

		key := fmt.Sprintf("%s:%s", tenantID, protoEntity.EntityId)

		// Temporarily update coordinator config
		oldConfig := s.coordinator.GetConfig()
		s.coordinator.UpdateConfig(coordinatorConfig)

		err = s.coordinator.Write(ctx, key, entityBytes)
		s.coordinator.UpdateConfig(oldConfig)

		if err != nil {
			failureCount++
			lastErr = err
			continue
		}

		// Add to unified index immediately (for demo/real-time indexing)
		if s.unifiedIndex != nil {
			_ = s.unifiedIndex.AddEntity(ent) // Ignore errors for now
		}

		successCount++
		entityIDs = append(entityIDs, protoEntity.EntityId)
	}

	errorMsg := ""
	if lastErr != nil {
		errorMsg = lastErr.Error()
	}

	return &pb.BatchPutEntitiesResponse{
		Success:      failureCount == 0,
		ErrorMessage: errorMsg,
		EntityIds:    entityIDs,
		SuccessCount: int32(successCount),
		FailureCount: int32(failureCount),
	}, nil
}

// Query performs a vector + graph search query.
func (s *Server) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	// Extract tenant ID from context
	tenantID, err := tenant.RequireTenant(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "tenant required: %v", err)
	}

	queryVector := req.QueryVector
	if len(queryVector) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "query_vector is required")
	}

	k := int(req.K)
	if k <= 0 {
		k = 10 // Default
	}

	expandFactor := int(req.ExpandFactor)
	if expandFactor <= 0 {
		expandFactor = 2 // Default
	}

	// Convert query vector from float32 to float32 (already correct type)
	queryVectorFloat32 := queryVector

	// Execute query
	results, err := s.queryExecutor.QueryWithOptions(ctx, queryVectorFloat32, k, expandFactor)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query execution failed: %v", err)
	}

	// Filter results by tenant ID
	filteredResults := make([]query.QueryResult, 0)
	for _, result := range results {
		if result.Entity != nil && result.Entity.GetTenantID() == tenantID {
			filteredResults = append(filteredResults, result)
		}
	}

	// Convert results to proto format
	protoResults := make([]*pb.QueryResult, 0, len(filteredResults))
	for _, result := range filteredResults {
		protoResult := &pb.QueryResult{
			EntityId: string(result.EntityID),
			Score:    result.Score,
		}

		// Optionally include entity data
		if result.Entity != nil {
			protoResult.Entity = s.entityToProto(result.Entity)
		}

		protoResults = append(protoResults, protoResult)
	}

	return &pb.QueryResponse{
		Results:      protoResults,
		TraceId:      req.TraceId,
		TotalResults: int32(len(protoResults)),
	}, nil
}

// RangeScan scans entities by key range with pagination.
func (s *Server) RangeScan(ctx context.Context, req *pb.RangeScanRequest) (*pb.RangeScanResponse, error) {
	if s.rangeScanner == nil {
		return nil, status.Errorf(codes.Unimplemented, "range scan not supported")
	}

	tenantID := req.TenantId
	if tenantID == "" {
		var err error
		tenantID, err = tenant.RequireTenant(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "tenant required: %v", err)
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	offset := 0
	if req.PageToken != "" {
		if v, err := strconv.Atoi(req.PageToken); err == nil && v >= 0 {
			offset = v
		}
	}

	// If target node specified and not local, forward via gRPC.
	if req.NodeId != "" && s.localNodeID != "" && req.NodeId != s.localNodeID {
		if s.nodeLookup == nil {
			return nil, status.Errorf(codes.Unimplemented, "node lookup not available for remote range scan")
		}
		addr, ok := s.nodeLookup(req.NodeId)
		if !ok {
			return nil, status.Errorf(codes.NotFound, "address for node %s not found", req.NodeId)
		}
		conn, err := grpc.Dial(addr, grpc.WithInsecure()) // assuming internal network; replace with TLS if needed
		if err != nil {
			return nil, status.Errorf(codes.Unavailable, "dial remote node %s failed: %v", req.NodeId, err)
		}
		defer conn.Close()
		client := pb.NewAlliDBClient(conn)
		fwdReq := &pb.RangeScanRequest{
			TenantId:  req.TenantId,
			StartKey:  req.StartKey,
			EndKey:    req.EndKey,
			Limit:     req.Limit,
			PageToken: req.PageToken,
			NodeId:    "", // avoid loops
		}
		return client.RangeScan(ctx, fwdReq)
	}

	data, err := s.rangeScanner.RangeScan(ctx, req.StartKey, req.EndKey, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "range scan failed: %v", err)
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if offset > len(keys) {
		offset = len(keys)
	}
	end := offset + limit
	if end > len(keys) {
		end = len(keys)
	}

	protoEntities := make([]*pb.Entity, 0, end-offset)
	for _, k := range keys[offset:end] {
		protoEntities = append(protoEntities, s.entityToProto(data[k]))
	}

	nextToken := ""
	if end < len(keys) {
		nextToken = strconv.Itoa(end)
	}

	return &pb.RangeScanResponse{
		Entities:      protoEntities,
		NextPageToken: nextToken,
	}, nil
}

// ReplicaRead reads raw data for replication.
func (s *Server) ReplicaRead(ctx context.Context, req *pb.ReplicaReadRequest) (*pb.ReplicaReadResponse, error) {
	if s.replicaKV == nil {
		return nil, status.Errorf(codes.Unimplemented, "replica read not supported")
	}
	if req.Key == "" {
		return nil, status.Errorf(codes.InvalidArgument, "key is required")
	}
	tenantID := req.TenantId
	if tenantID == "" {
		var err error
		tenantID, err = tenant.RequireTenant(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "tenant required: %v", err)
		}
	}
	val, err := s.replicaKV.ReplicaRead(ctx, req.Key, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "replica read failed: %v", err)
	}
	return &pb.ReplicaReadResponse{Value: val}, nil
}

// ReplicaWrite writes raw data for replication.
func (s *Server) ReplicaWrite(ctx context.Context, req *pb.ReplicaWriteRequest) (*pb.ReplicaWriteResponse, error) {
	if s.replicaKV == nil {
		return nil, status.Errorf(codes.Unimplemented, "replica write not supported")
	}
	if req.Key == "" {
		return nil, status.Errorf(codes.InvalidArgument, "key is required")
	}
	tenantID := req.TenantId
	if tenantID == "" {
		var err error
		tenantID, err = tenant.RequireTenant(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "tenant required: %v", err)
		}
	}
	if err := s.replicaKV.ReplicaWrite(ctx, req.Key, tenantID, req.Value); err != nil {
		return nil, status.Errorf(codes.Internal, "replica write failed: %v", err)
	}
	return &pb.ReplicaWriteResponse{Success: true}, nil
}

// HealthCheck checks the health of the service.
func (s *Server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// Simple health check - verify coordinator and query executor are available
	healthy := s.coordinator != nil && s.queryExecutor != nil

	statusMsg := "healthy"
	if !healthy {
		statusMsg = "unhealthy"
	}

	return &pb.HealthCheckResponse{
		Healthy:        healthy,
		Status:         statusMsg,
		TimestampNanos: time.Now().UnixNano(),
	}, nil
}

// getCoordinatorConfig converts a consistency level to coordinator configuration.
func (s *Server) getCoordinatorConfig(level pb.ConsistencyLevel) cluster.Config {
	config := s.coordinator.GetConfig()

	switch level {
	case pb.ConsistencyLevel_CONSISTENCY_ONE:
		config.ReadQuorum = 1
		config.WriteQuorum = 1
	case pb.ConsistencyLevel_CONSISTENCY_QUORUM:
		// Use default quorum (already set in config)
		// config.ReadQuorum and config.WriteQuorum remain unchanged
	case pb.ConsistencyLevel_CONSISTENCY_ALL:
		config.ReadQuorum = config.ReplicaCount
		config.WriteQuorum = config.ReplicaCount
	default:
		// Default to quorum
	}

	return config
}

// protoToEntity converts a proto Entity to a UnifiedEntity.
func (s *Server) protoToEntity(protoEntity *pb.Entity, tenantID string) *entity.UnifiedEntity {
	ent := entity.NewUnifiedEntity(
		entity.EntityID(protoEntity.EntityId),
		tenantID,
		entity.EntityType(protoEntity.EntityType),
	)

	// Add embeddings
	for _, protoEmbedding := range protoEntity.Embeddings {
		if protoEmbedding != nil {
			vector := protoEmbedding.Vector
			ent.AddEmbedding(vector)
		}
	}

	// Add incoming edges
	for _, protoEdge := range protoEntity.IncomingEdges {
		if protoEdge != nil {
			edge := entity.GraphEdge{
				FromEntity: entity.EntityID(protoEdge.FromEntity),
				ToEntity:   entity.EntityID(protoEdge.ToEntity),
				Relation:   entity.Relation(protoEdge.Relation),
				Weight:     protoEdge.Weight,
				Timestamp:  time.Unix(0, protoEdge.TimestampNanos),
			}
			ent.AddIncomingEdge(edge)
		}
	}

	// Add outgoing edges
	for _, protoEdge := range protoEntity.OutgoingEdges {
		if protoEdge != nil {
			edge := entity.GraphEdge{
				FromEntity: entity.EntityID(protoEdge.FromEntity),
				ToEntity:   entity.EntityID(protoEdge.ToEntity),
				Relation:   entity.Relation(protoEdge.Relation),
				Weight:     protoEdge.Weight,
				Timestamp:  time.Unix(0, protoEdge.TimestampNanos),
			}
			ent.AddOutgoingEdge(edge)
		}
	}

	// Add chunks
	for _, protoChunk := range protoEntity.Chunks {
		if protoChunk != nil {
			chunk := entity.TextChunk{
				ChunkID:  entity.ChunkID(protoChunk.ChunkId),
				Text:     protoChunk.Text,
				Metadata: protoChunk.Metadata,
			}
			ent.AddChunk(chunk)
		}
	}

	// Set importance
	ent.SetImportance(protoEntity.Importance)

	return ent
}

// entityToProto converts a UnifiedEntity to a proto Entity.
func (s *Server) entityToProto(ent *entity.UnifiedEntity) *pb.Entity {
	embeddings := ent.GetEmbeddings()
	protoEmbeddings := make([]*pb.VectorEmbedding, 0, len(embeddings))
	for _, emb := range embeddings {
		protoEmbeddings = append(protoEmbeddings, &pb.VectorEmbedding{
			Version:        emb.Version,
			Vector:         emb.Vector,
			TimestampNanos: emb.Timestamp.UnixNano(),
		})
	}

	incomingEdges := ent.GetIncomingEdges()
	protoIncomingEdges := make([]*pb.GraphEdge, 0, len(incomingEdges))
	for _, edge := range incomingEdges {
		protoIncomingEdges = append(protoIncomingEdges, &pb.GraphEdge{
			FromEntity:     string(edge.FromEntity),
			ToEntity:       string(edge.ToEntity),
			Relation:       string(edge.Relation),
			Weight:         edge.Weight,
			TimestampNanos: edge.Timestamp.UnixNano(),
		})
	}

	outgoingEdges := ent.GetOutgoingEdges()
	protoOutgoingEdges := make([]*pb.GraphEdge, 0, len(outgoingEdges))
	for _, edge := range outgoingEdges {
		protoOutgoingEdges = append(protoOutgoingEdges, &pb.GraphEdge{
			FromEntity:     string(edge.FromEntity),
			ToEntity:       string(edge.ToEntity),
			Relation:       string(edge.Relation),
			Weight:         edge.Weight,
			TimestampNanos: edge.Timestamp.UnixNano(),
		})
	}

	chunks := ent.GetChunks()
	protoChunks := make([]*pb.TextChunk, 0, len(chunks))
	for _, chunk := range chunks {
		protoChunks = append(protoChunks, &pb.TextChunk{
			ChunkId:  string(chunk.ChunkID),
			Text:     chunk.Text,
			Metadata: chunk.Metadata,
		})
	}

	return &pb.Entity{
		EntityId:        string(ent.GetEntityID()),
		TenantId:        ent.GetTenantID(),
		EntityType:      string(ent.GetEntityType()),
		Embeddings:      protoEmbeddings,
		IncomingEdges:   protoIncomingEdges,
		OutgoingEdges:   protoOutgoingEdges,
		Chunks:          protoChunks,
		Importance:      ent.GetImportance(),
		CreatedAtNanos:  ent.GetCreatedAt().UnixNano(),
		UpdatedAtNanos:  ent.GetUpdatedAt().UnixNano(),
	}
}
