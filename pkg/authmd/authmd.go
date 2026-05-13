// Package authmd implements JWT minting/parsing and gRPC metadata helpers
// used by both the gateway (verify + propagate) and microservices (read
// caller identity from incoming metadata).
//
// JWT shape: HS256 with the user id in the custom claim "uid". The JWT id
// ("jti") doubles as a Redis session key so tokens can be revoked by simply
// deleting the key.
package authmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
)

// MD keys used to propagate identity over gRPC. They are lower-cased per the
// gRPC metadata convention.
const (
	MDUserID = "x-user-id"
	MDJTI    = "x-jti"
)

// Signer mints and verifies tokens.
type Signer struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// Claims is the structured payload of every issued token.
type Claims struct {
	UserID int64 `json:"uid"`
	jwt.RegisteredClaims
}

// New returns a Signer or an error if the inputs are invalid.
func New(secret string, ttl time.Duration, issuer string) (*Signer, error) {
	if secret == "" {
		return nil, errors.New("authmd: empty jwt secret")
	}
	if ttl <= 0 {
		return nil, errors.New("authmd: jwt ttl must be > 0")
	}
	return &Signer{secret: []byte(secret), ttl: ttl, issuer: issuer}, nil
}

// TTL exposes the configured lifetime.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Issue creates a new token for userID. The returned jti is intended to be
// stored in Redis as a revocation/session key.
func (s *Signer) Issue(userID int64) (token, jti string, expiresAt time.Time, err error) {
	jti, err = randomHex(16)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("authmd: jti: %w", err)
	}
	now := time.Now()
	expiresAt = now.Add(s.ttl)

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("authmd: sign: %w", err)
	}
	return signed, jti, expiresAt, nil
}

// Parse validates token and returns its claims.
func (s *Signer) Parse(token string) (*Claims, error) {
	var c Claims
	t, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("authmd: unexpected signing method %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, errors.New("authmd: invalid token")
	}
	return &c, nil
}

// Inject puts uid/jti onto the outgoing gRPC metadata of ctx.
func Inject(ctx context.Context, userID int64, jti string) context.Context {
	md := metadata.Pairs(
		MDUserID, fmt.Sprintf("%d", userID),
		MDJTI, jti,
	)
	return metadata.AppendToOutgoingContext(ctx, MDUserID, md.Get(MDUserID)[0], MDJTI, jti)
}

// Identity is the caller info extracted from incoming gRPC metadata.
type Identity struct {
	UserID int64
	JTI    string
}

// From extracts the identity from the incoming gRPC metadata of ctx. The
// second return is false when the metadata is missing or malformed.
func From(ctx context.Context) (Identity, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Identity{}, false
	}
	uids := md.Get(MDUserID)
	jtis := md.Get(MDJTI)
	if len(uids) == 0 || len(jtis) == 0 {
		return Identity{}, false
	}
	var uid int64
	if _, err := fmt.Sscan(uids[0], &uid); err != nil {
		return Identity{}, false
	}
	return Identity{UserID: uid, JTI: jtis[0]}, true
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
