package main

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ctxKey is a private context key type to avoid collisions.
type ctxKey int

const claimsCtxKey ctxKey = iota

// withClaims stores the authenticated claims in the context.
func withClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsCtxKey, c)
}

// claimsFromContext retrieves the authenticated claims from the context.
func claimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey).(*Claims)
	return c, ok
}

// publicMethods are reachable without a session token.
var publicMethods = map[string]bool{
	peoplev1.PeopleService_GetChallenge_FullMethodName: true,
	peoplev1.PeopleService_Login_FullMethodName:        true,
}

// requiredPermission is the RBAC capability gating a method.
type requiredPermission struct {
	resource string
	action   string
}

// routePermissions maps an authenticated method to the permission a caller must
// hold. Methods absent from this map require a valid token but no specific
// permission (e.g. Logout).
var routePermissions = map[string]requiredPermission{
	peoplev1.PeopleService_CheckPermission_FullMethodName: {resource: "permission", action: "read"},
}

// NewAuthInterceptor builds a unary interceptor that authenticates JWT sessions
// and enforces per-method RBAC.
func NewAuthInterceptor(auth *Authenticator, repo *Repository, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		token, err := bearerFromContext(ctx)
		if err != nil {
			return nil, err
		}
		claims, err := auth.ParseToken(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		ctx = withClaims(ctx, claims)

		if perm, ok := routePermissions[info.FullMethod]; ok {
			userID, err := strconv.ParseInt(claims.Subject, 10, 64)
			if err != nil {
				return nil, status.Error(codes.Unauthenticated, "malformed token subject")
			}
			allowed, err := repo.HasPermission(ctx, userID, perm.resource, perm.action)
			if err != nil {
				logger.Error("people: rbac check", "method", info.FullMethod, "err", err)
				return nil, status.Error(codes.Internal, "authorization check failed")
			}
			if !allowed {
				return nil, status.Error(codes.PermissionDenied, "permission denied")
			}
		}

		return handler(ctx, req)
	}
}

// bearerFromContext extracts the bearer token from the gRPC "authorization"
// metadata header.
func bearerFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	const prefix = "bearer "
	auth := values[0]
	if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", status.Error(codes.Unauthenticated, "authorization header must be a Bearer token")
	}
	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "empty bearer token")
	}
	return token, nil
}
