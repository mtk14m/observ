package otlp

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// validBearer reports whether an Authorization header carries the token.
// The comparison takes constant time.
func validBearer(header, token string) bool {
	got, ok := strings.CutPrefix(header, "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// TokenInterceptor rejects OTLP/gRPC calls that do not carry
// "authorization: Bearer <token>".
func TokenInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		for _, v := range md.Get("authorization") {
			if validBearer(v, token) {
				return next(ctx, req)
			}
		}
		return nil, status.Error(codes.Unauthenticated, "missing or invalid ingest token")
	}
}
