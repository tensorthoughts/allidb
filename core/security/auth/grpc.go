// +build grpc

package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCAuthInterceptor creates a gRPC interceptor for API key authentication.
func GRPCAuthInterceptor(store *APIKeyStore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract metadata from context
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		// Get API key from metadata
		apiKeys := md.Get(strings.ToLower(APIKeyHeader))
		if len(apiKeys) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing API key")
		}

		apiKey := apiKeys[0]
		if apiKey == "" {
			return nil, status.Errorf(codes.Unauthenticated, "empty API key")
		}

		// Get tenant ID from store
		tenantID, ok := store.GetTenantID(apiKey)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "invalid API key")
		}

		// Inject tenant ID into context
		ctx = WithTenantID(ctx, tenantID)

		return handler(ctx, req)
	}
}

// GRPCStreamAuthInterceptor creates a gRPC stream interceptor for API key authentication.
func GRPCStreamAuthInterceptor(store *APIKeyStore) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Extract metadata from context
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		// Get API key from metadata
		apiKeys := md.Get(strings.ToLower(APIKeyHeader))
		if len(apiKeys) == 0 {
			return status.Errorf(codes.Unauthenticated, "missing API key")
		}

		apiKey := apiKeys[0]
		if apiKey == "" {
			return status.Errorf(codes.Unauthenticated, "empty API key")
		}

		// Get tenant ID from store
		tenantID, ok := store.GetTenantID(apiKey)
		if !ok {
			return status.Errorf(codes.Unauthenticated, "invalid API key")
		}

		// Inject tenant ID into context
		ctx := WithTenantID(ss.Context(), tenantID)

		// Create a new stream with the updated context
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream wraps grpc.ServerStream to override Context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

