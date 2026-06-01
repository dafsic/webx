package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Claims mirrors the people service's JWT payload. The gateway only needs the
// standard registered claims (signature + expiry) to authenticate a request;
// the custom address claim is kept so the struct stays compatible.
type Claims struct {
	Address string `json:"addr"`
	jwt.RegisteredClaims
}

// publicAPIPaths are proxied routes reachable without a session token. The docs
// endpoints (/docs, /openapi.json) are registered as more specific mux patterns
// and therefore bypass this middleware automatically.
var publicAPIPaths = map[string]bool{
	"/api/v1/people:getChallenge": true,
	"/api/v1/people:login":        true,
}

// isPublicRequest reports whether a request may bypass authentication. Besides
// the explicitly public endpoints, customers interact without logging in to:
//   - browse the menu: GET under /api/v1/catalog/items
//   - place an order:  POST /api/v1/orders
//   - track an order:  GET /api/v1/orders/{id}
//
// Staff-only routes (listing all orders, mutating the menu, updating an order's
// status) still require a token.
func isPublicRequest(r *http.Request) bool {
	if publicAPIPaths[r.URL.Path] {
		return true
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/catalog/items") {
		return true
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders" {
		return true
	}
	// GET /api/v1/orders/{id} is public, but GET /api/v1/orders (the list) is not.
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/orders/") {
		return true
	}
	return false
}

// authMiddleware is the gateway's centralized authentication layer. It rejects
// requests to protected routes that lack a valid HS256 JWT before they are
// proxied to a backend microservice. Authorization is left to each service's
// own RBAC interceptor; the bearer token is forwarded unchanged.
func authMiddleware(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		token, err := bearerToken(r)
		if err != nil {
			writeAuthError(w, err.Error())
			return
		}
		if err := validateToken(secret, token); err != nil {
			writeAuthError(w, "invalid or expired token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the JWT from the "Authorization: Bearer <jwt>" header.
func bearerToken(r *http.Request) (string, error) {
	const prefix = "bearer "
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("missing authorization header")
	}
	if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", fmt.Errorf("authorization header must be a Bearer token")
	}
	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return "", fmt.Errorf("empty bearer token")
	}
	return token, nil
}

// validateToken verifies the token's HS256 signature and expiry.
func validateToken(secret []byte, token string) error {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return fmt.Errorf("invalid token")
	}
	return nil
}

// writeAuthError emits a grpc-gateway-style JSON error with HTTP 401 so clients
// see a consistent error shape across authenticated and proxied responses.
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	// gRPC code 16 == UNAUTHENTICATED.
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    16,
		"message": message,
	})
}
