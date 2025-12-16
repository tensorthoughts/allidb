package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/tensorthoughts25/allidb/api/grpc/pb"
	"github.com/tensorthoughts25/allidb/core/security/tenant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"github.com/tensorthoughts25/allidb/core/observability"
)

var logger = observability.DefaultLogger().WithField("component", "handlers")
// Gateway is a REST gateway that translates HTTP requests to gRPC calls.
type Gateway struct {
	grpcClient pb.AlliDBClient
	conn       *grpc.ClientConn
}

// Config holds configuration for the HTTP gateway.
type Config struct {
	GRPCAddress string // Address of the gRPC server (e.g., "localhost:50051")
	GRPCConn    *grpc.ClientConn // Optional: pre-established gRPC connection
}

// NewGateway creates a new REST gateway.
func NewGateway(config Config) (*Gateway, error) {
	var conn *grpc.ClientConn
	var err error

	if config.GRPCConn != nil {
		conn = config.GRPCConn
	} else if config.GRPCAddress != "" {
		conn, err = grpc.Dial(config.GRPCAddress, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("either GRPCAddress or GRPCConn must be provided")
	}

	client := pb.NewAlliDBClient(conn)

	return &Gateway{
		grpcClient: client,
		conn:       conn,
	}, nil
}

// Close closes the gRPC connection.
func (g *Gateway) Close() error {
	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}

// PutEntityHandler handles POST /entity requests.
func (g *Gateway) PutEntityHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON request
	var req struct {
		Entity           *pb.Entity          `json:"entity"`
		ConsistencyLevel string              `json:"consistency_level,omitempty"`
		TraceID          string              `json:"trace_id,omitempty"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Entity == nil {
		http.Error(w, "entity is required", http.StatusBadRequest)
		return
	}

	// Convert consistency level
	consistencyLevel := parseConsistencyLevel(req.ConsistencyLevel)

	// Create gRPC request
	grpcReq := &pb.PutEntityRequest{
		Entity:           req.Entity,
		ConsistencyLevel: consistencyLevel,
		TraceId:          req.TraceID,
	}

	// Get tenant ID from context (set by auth middleware)
	tenantID, ok := tenant.TenantFromContext(r.Context())
	if !ok {
		logger.Error("Tenant not found in context", errors.New("tenant not found in context"))
		http.Error(w, "Tenant not found in context", http.StatusInternalServerError)
		return
	}

	// Create gRPC context with tenant and API key
	ctx := context.WithValue(r.Context(), "http_request", r)
	ctx = g.createGRPCContext(ctx, tenantID)

	// Call gRPC service
	grpcResp, err := g.grpcClient.PutEntity(ctx, grpcReq)
	if err != nil {
		g.writeGRPCError(w, err)
		return
	}

	// Write JSON response
	g.writeJSONResponse(w, http.StatusOK, grpcResp)
}

// BatchPutEntitiesHandler handles POST /entities/batch requests.
func (g *Gateway) BatchPutEntitiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON request
	var req struct {
		Entities         []*pb.Entity `json:"entities"`
		ConsistencyLevel string       `json:"consistency_level,omitempty"`
		TraceID          string       `json:"trace_id,omitempty"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Entities) == 0 {
		http.Error(w, "at least one entity is required", http.StatusBadRequest)
		return
	}

	// Convert consistency level
	consistencyLevel := parseConsistencyLevel(req.ConsistencyLevel)

	// Create gRPC request
	grpcReq := &pb.BatchPutEntitiesRequest{
		Entities:         req.Entities,
		ConsistencyLevel: consistencyLevel,
		TraceId:          req.TraceID,
	}

	// Get tenant ID from context
	tenantID, ok := tenant.TenantFromContext(r.Context())
	if !ok {
		http.Error(w, "Tenant not found in context", http.StatusInternalServerError)
		return
	}

	// Create gRPC context
	ctx := context.WithValue(r.Context(), "http_request", r)
	ctx = g.createGRPCContext(ctx, tenantID)

	// Call gRPC service
	grpcResp, err := g.grpcClient.BatchPutEntities(ctx, grpcReq)
	if err != nil {
		g.writeGRPCError(w, err)
		return
	}

	// Determine HTTP status code based on success
	statusCode := http.StatusOK
	if !grpcResp.Success {
		statusCode = http.StatusPartialContent // 206 for partial success
	}

	// Write JSON response
	g.writeJSONResponse(w, statusCode, grpcResp)
}

// QueryHandler handles POST /query requests.
func (g *Gateway) QueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON request
	var req struct {
		QueryVector      []float32 `json:"query_vector"`
		K                int32     `json:"k,omitempty"`
		ExpandFactor     int32     `json:"expand_factor,omitempty"`
		TraceID          string    `json:"trace_id,omitempty"`
		ConsistencyLevel string    `json:"consistency_level,omitempty"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.QueryVector) == 0 {
		http.Error(w, "query_vector is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.K <= 0 {
		req.K = 10
	}
	if req.ExpandFactor <= 0 {
		req.ExpandFactor = 2
	}

	// Convert consistency level
	consistencyLevel := parseConsistencyLevel(req.ConsistencyLevel)

	// Create gRPC request
	grpcReq := &pb.QueryRequest{
		QueryVector:      req.QueryVector,
		K:                req.K,
		ExpandFactor:     req.ExpandFactor,
		TraceId:          req.TraceID,
		ConsistencyLevel: consistencyLevel,
	}

	// Get tenant ID from context
	tenantID, ok := tenant.TenantFromContext(r.Context())
	if !ok {
		http.Error(w, "Tenant not found in context", http.StatusInternalServerError)
		return
	}

	// Create gRPC context
	ctx := context.WithValue(r.Context(), "http_request", r)
	ctx = g.createGRPCContext(ctx, tenantID)

	// Call gRPC service
	grpcResp, err := g.grpcClient.Query(ctx, grpcReq)
	if err != nil {
		g.writeGRPCError(w, err)
		return
	}

	// Write JSON response
	g.writeJSONResponse(w, http.StatusOK, grpcResp)
}

// HealthCheckHandler handles GET /health requests.
func (g *Gateway) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create gRPC request
	grpcReq := &pb.HealthCheckRequest{}

	// Call gRPC service (no auth required for health check)
	ctx := r.Context()
	grpcResp, err := g.grpcClient.HealthCheck(ctx, grpcReq)
	if err != nil {
		g.writeGRPCError(w, err)
		return
	}

	// Determine HTTP status code based on health
	statusCode := http.StatusOK
	if !grpcResp.Healthy {
		statusCode = http.StatusServiceUnavailable
	}

	// Write JSON response
	g.writeJSONResponse(w, statusCode, grpcResp)
}

// createGRPCContext creates a gRPC context with tenant ID and API key metadata.
func (g *Gateway) createGRPCContext(ctx context.Context, tenantID string) context.Context {
	// Get the original HTTP request from context if available
	// We need to extract the API key from the request headers
	var apiKey string
	if req, ok := ctx.Value("http_request").(*http.Request); ok {
		apiKey = req.Header.Get("X-ALLIDB-API-KEY")
	}

	// Create metadata with tenant ID and API key
	md := metadata.New(map[string]string{})
	if apiKey != "" {
		md.Set("x-allidb-api-key", apiKey)
	}

	// Attach metadata
	ctxWithMD := metadata.NewOutgoingContext(ctx, md)

	// Also attach tenant ID directly to context so gRPC handlers that call
	// tenant.RequireTenant() see it even if auth interceptors are not used.
	if tenantID != "" {
		ctxWithMD = tenant.WithTenant(ctxWithMD, tenantID)
	}

	return ctxWithMD
}

// parseConsistencyLevel parses a consistency level string to proto enum.
func parseConsistencyLevel(level string) pb.ConsistencyLevel {
	switch level {
	case "ONE", "one":
		return pb.ConsistencyLevel_CONSISTENCY_ONE
	case "QUORUM", "quorum":
		return pb.ConsistencyLevel_CONSISTENCY_QUORUM
	case "ALL", "all":
		return pb.ConsistencyLevel_CONSISTENCY_ALL
	default:
		return pb.ConsistencyLevel_CONSISTENCY_QUORUM // Default
	}
}

// writeJSONResponse writes a JSON response.
func (g *Gateway) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, we can't change the status code, but we can log
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// writeGRPCError writes a gRPC error as an HTTP error.
func (g *Gateway) writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Map gRPC status codes to HTTP status codes
	httpStatus := g.grpcToHTTPStatus(st.Code())
	
	// Create error response
	errorResp := map[string]interface{}{
		"error":   st.Message(),
		"code":    st.Code().String(),
		"details": st.Details(),
	}

	g.writeJSONResponse(w, httpStatus, errorResp)
}

// grpcToHTTPStatus maps gRPC status codes to HTTP status codes.
func (g *Gateway) grpcToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// RegisterRoutes registers all HTTP routes with the given mux.
func (g *Gateway) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/entity", g.PutEntityHandler)
	mux.HandleFunc("/entities/batch", g.BatchPutEntitiesHandler)
	mux.HandleFunc("/query", g.QueryHandler)
	mux.HandleFunc("/health", g.HealthCheckHandler)
}

