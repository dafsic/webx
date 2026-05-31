package main

import "time"

// Module / flag identifiers and defaults for the people service.
const (
	moduleName = "people"

	defaultGRPCAddr = ":50051"
	defaultJWTTTL   = 24 * time.Hour
	defaultNonceTTL = 5 * time.Minute

	// Redis key prefixes.
	nonceKeyPrefix   = "people:nonce:"   // nonce per address, consumed on login
	revokedKeyPrefix = "people:revoked:" // revoked JWT ids (jti) until expiry
)

// Config is the resolved configuration for the people service.
type Config struct {
	// GRPCAddr is the listen address of the gRPC server.
	GRPCAddr string
	// JWTSecret signs and verifies session tokens (HS256). Required.
	JWTSecret string
	// JWTTTL is the lifetime of an issued session token.
	JWTTTL time.Duration
	// NonceTTL is how long a login challenge nonce remains valid.
	NonceTTL time.Duration
}
