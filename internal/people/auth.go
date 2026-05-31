package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
	goredis "github.com/redis/go-redis/v9"
)

// ErrChallengeNotFound is returned when no (valid) nonce exists for an address,
// e.g. it expired, was already consumed, or GetChallenge was never called.
var ErrChallengeNotFound = errors.New("people: challenge not found or expired")

// ErrBadSignature is returned when the supplied signature does not recover to
// the claimed address.
var ErrBadSignature = errors.New("people: signature does not match address")

// Claims is the JWT payload for an authenticated session.
type Claims struct {
	Address string `json:"addr"`
	jwt.RegisteredClaims
}

// Authenticator handles login challenges, EVM signature verification and JWT
// session tokens. Nonces and the revocation list are kept in Redis.
type Authenticator struct {
	rdb    goredis.UniversalClient
	cfg    *Config
	logger *slog.Logger
}

// NewAuthenticator constructs an Authenticator.
func NewAuthenticator(rdb goredis.UniversalClient, cfg *Config, logger *slog.Logger) *Authenticator {
	return &Authenticator{rdb: rdb, cfg: cfg, logger: logger}
}

// IssueChallenge generates a fresh random nonce for address, stores it in Redis
// with the configured TTL and returns it. The client must sign the message
// produced by SignMessage(address, nonce) and present the signature to Login.
func (a *Authenticator) IssueChallenge(ctx context.Context, address string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("people: generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(b[:])
	if err := a.rdb.Set(ctx, nonceKeyPrefix+address, nonce, a.cfg.NonceTTL).Err(); err != nil {
		return "", fmt.Errorf("people: store nonce: %w", err)
	}
	return nonce, nil
}

// SignMessage is the canonical human-readable message the wallet signs with
// personal_sign (EIP-191). It binds the nonce to the address.
func SignMessage(address, nonce string) string {
	return fmt.Sprintf(
		"Webx wants you to sign in with your Ethereum account:\n%s\n\nNonce: %s",
		address, nonce,
	)
}

// VerifyLogin fetches the stored nonce for address, verifies the EIP-191
// signature over SignMessage(address, nonce) and, on success, consumes the
// nonce so it cannot be replayed.
func (a *Authenticator) VerifyLogin(ctx context.Context, address, signature string) error {
	nonce, err := a.rdb.Get(ctx, nonceKeyPrefix+address).Result()
	switch {
	case errors.Is(err, goredis.Nil):
		return ErrChallengeNotFound
	case err != nil:
		return fmt.Errorf("people: load nonce: %w", err)
	}

	if err := verifySignature(address, SignMessage(address, nonce), signature); err != nil {
		return err
	}

	// One-time use: consume the nonce. A delete failure is non-fatal for the
	// login itself but worth logging since it weakens replay protection.
	if err := a.rdb.Del(ctx, nonceKeyPrefix+address).Err(); err != nil {
		a.logger.Warn("people: failed to consume nonce", "address", address, "err", err)
	}
	return nil
}

// verifySignature recovers the signer of an EIP-191 personal_sign signature and
// compares it (case-insensitively) to the claimed address.
func verifySignature(address, message, signature string) error {
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return fmt.Errorf("%w: malformed hex", ErrBadSignature)
	}
	if len(sig) != 65 {
		return fmt.Errorf("%w: expected 65 bytes, got %d", ErrBadSignature, len(sig))
	}
	// Normalize the recovery id (v): wallets emit 27/28, go-ethereum wants 0/1.
	if sig[64] == 27 || sig[64] == 28 {
		sig[64] -= 27
	}

	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	recovered := crypto.PubkeyToAddress(*pub)
	if !strings.EqualFold(recovered.Hex(), address) {
		return ErrBadSignature
	}
	return nil
}

// IssueToken mints a signed HS256 JWT for the authenticated user and returns it
// together with its absolute expiry.
func (a *Authenticator) IssueToken(userID int64, address string) (token string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(a.cfg.JWTTTL)
	claims := &Claims{
		Address: address,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ID:        newJTI(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(a.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("people: sign token: %w", err)
	}
	return token, expiresAt, nil
}

// ParseToken validates a token's signature and expiry and ensures it has not
// been revoked, returning its claims.
func (a *Authenticator) ParseToken(ctx context.Context, token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(a.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("people: invalid token: %w", err)
	}

	revoked, err := a.rdb.Exists(ctx, revokedKeyPrefix+claims.ID).Result()
	if err != nil {
		return nil, fmt.Errorf("people: check revocation: %w", err)
	}
	if revoked > 0 {
		return nil, errors.New("people: token revoked")
	}
	return claims, nil
}

// Revoke adds the token's id to the Redis denylist until its natural expiry so
// the same token cannot be reused after logout.
func (a *Authenticator) Revoke(ctx context.Context, claims *Claims) error {
	if claims.ExpiresAt == nil {
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // already expired, nothing to deny
	}
	if err := a.rdb.Set(ctx, revokedKeyPrefix+claims.ID, "1", ttl).Err(); err != nil {
		return fmt.Errorf("people: revoke token: %w", err)
	}
	return nil
}

// newJTI returns a random token id used for revocation tracking.
func newJTI() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
