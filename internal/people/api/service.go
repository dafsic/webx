// Package api implements the business logic of the people service.
//
// The single Service type owns all three use cases:
//   - GetChallenge – issues a time-limited EIP-712 nonce for a wallet address.
//   - Login        – verifies an EIP-712 signature, auto-registers the user on
//     first login, and returns a JWT session.
//   - Logout       – revokes the JWT session by deleting its jti from Redis.
//   - CheckPermission – answers RBAC queries.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dafsic/webx/internal/people/dao"
	"github.com/dafsic/webx/internal/people/model"
	"github.com/dafsic/webx/pkg/authmd"
	"github.com/dafsic/webx/pkg/eip712"
	"github.com/dafsic/webx/pkg/grpcerr"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTokenPrefix     = "people:token:"
	defaultChallengePrefix = "people:nonce:"
	challengeTTL           = 5 * time.Minute
)

// LoginResult is the output of a successful Login call.
type LoginResult struct {
	Token     string
	JTI       string
	ExpiresAt time.Time
	UserID    int64
	Address   string
	Nickname  string
	AvatarURL string
	Email     string
	Roles     []model.Role
	IsNew     bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Params bundles all dependencies for Service.
type Params struct {
	Users           dao.UserDAO
	Roles           dao.RoleDAO
	Signer          *authmd.Signer
	Redis           redis.UniversalClient
	Domain          eip712.Domain
	TokenPrefix     string
	ChallengePrefix string
	Logger          *slog.Logger
}

// Service implements all people business logic.
type Service struct {
	users           dao.UserDAO
	roles           dao.RoleDAO
	signer          *authmd.Signer
	rdb             redis.UniversalClient
	domain          eip712.Domain
	tokenPrefix     string
	challengePrefix string
	logger          *slog.Logger
}

// New constructs a Service.
func NewService(p Params) *Service {
	if p.TokenPrefix == "" {
		p.TokenPrefix = defaultTokenPrefix
	}
	if p.ChallengePrefix == "" {
		p.ChallengePrefix = defaultChallengePrefix
	}
	return &Service{
		users:           p.Users,
		roles:           p.Roles,
		signer:          p.Signer,
		rdb:             p.Redis,
		domain:          p.Domain,
		tokenPrefix:     p.TokenPrefix,
		challengePrefix: p.ChallengePrefix,
		logger:          p.Logger,
	}
}

// ── GetChallenge ───────────────────────────────────────────────────────────

// GetChallenge generates a random nonce bound to the given wallet address and
// stores it in Redis for challengeTTL (5 min). The client must sign the nonce
// with eth_signTypedData_v4 and pass the resulting signature to Login.
func (s *Service) GetChallenge(ctx context.Context, address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("%w: address required", grpcerr.ErrInvalidArgument)
	}
	nonce, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("people: generate nonce: %w", err)
	}
	key := s.challengePrefix + strings.ToLower(address)
	if err := s.rdb.Set(ctx, key, nonce, challengeTTL).Err(); err != nil {
		return "", fmt.Errorf("people: store nonce: %w", err)
	}
	s.logger.Debug("people: challenge issued", "address", address)
	return nonce, nil
}

// ── Login ──────────────────────────────────────────────────────────────────

// Login verifies an EIP-712 signature for the previously-issued nonce,
// auto-registers the wallet on first login, and returns a JWT session.
func (s *Service) Login(ctx context.Context, address, signature string) (*LoginResult, error) {
	if address == "" || signature == "" {
		return nil, fmt.Errorf("%w: address and signature required", grpcerr.ErrInvalidArgument)
	}
	addr := strings.ToLower(address)

	// 1. Fetch and immediately delete the nonce (single-use).
	key := s.challengePrefix + addr
	nonce, err := s.rdb.GetDel(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("%w: challenge not found or expired", grpcerr.ErrUnauthenticated)
		}
		return nil, fmt.Errorf("people: fetch nonce: %w", err)
	}

	// 2. Verify EIP-712 signature.
	digest := s.domain.LoginDigest(nonce)
	recovered, err := eip712.RecoverAddress(digest, signature)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature: %v", grpcerr.ErrUnauthenticated, err)
	}
	if recovered != addr {
		s.logger.Warn("people: address mismatch",
			"claimed", addr, "recovered", recovered)
		return nil, fmt.Errorf("%w: signature address mismatch", grpcerr.ErrUnauthenticated)
	}

	// 3. Get or auto-create the user.
	user, isNew, err := s.users.GetOrCreateByAddress(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("people: resolve user: %w", err)
	}

	// 4. Mint JWT and persist session in Redis.
	token, jti, expiresAt, err := s.signer.Issue(user.ID)
	if err != nil {
		return nil, fmt.Errorf("people: issue token: %w", err)
	}
	if err := s.rdb.Set(ctx, s.tokenPrefix+jti, user.ID, time.Until(expiresAt)).Err(); err != nil {
		return nil, fmt.Errorf("people: persist token: %w", err)
	}

	// 5. Load RBAC roles (best-effort: new users have none yet).
	roles, err := s.roles.GetRolesByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("people: load roles: %w", err)
	}

	s.logger.Info("people: login ok",
		"user_id", user.ID, "address", addr, "new_user", isNew,
		"jti", jti, "expires_at", expiresAt.Unix())

	return &LoginResult{
		Token:     token,
		JTI:       jti,
		ExpiresAt: expiresAt,
		UserID:    user.ID,
		Address:   user.Address,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
		Email:     user.Email,
		Roles:     roles,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// ── Logout ─────────────────────────────────────────────────────────────────

// Logout revokes the JWT session by deleting its jti from Redis.
func (s *Service) Logout(ctx context.Context, userID int64, jti string) error {
	if jti == "" {
		return fmt.Errorf("%w: jti required", grpcerr.ErrInvalidArgument)
	}
	n, err := s.rdb.Del(ctx, s.tokenPrefix+jti).Result()
	if err != nil {
		return fmt.Errorf("people: logout: %w", err)
	}
	s.logger.Info("people: logout", "user_id", userID, "jti", jti, "deleted", n > 0)
	return nil
}

// ── CheckPermission ────────────────────────────────────────────────────────

// CheckPermission reports whether userID holds the given resource+action pair.
func (s *Service) CheckPermission(ctx context.Context, userID int64, resource, action string) (bool, error) {
	if userID == 0 {
		return false, fmt.Errorf("%w: user_id required", grpcerr.ErrInvalidArgument)
	}
	if resource == "" || action == "" {
		return false, fmt.Errorf("%w: resource and action required", grpcerr.ErrInvalidArgument)
	}
	allowed, err := s.roles.HasPermission(ctx, userID, resource, action)
	if err != nil {
		return false, fmt.Errorf("people: check permission: %w", err)
	}
	s.logger.Debug("people: check permission",
		"user_id", userID, "resource", resource, "action", action, "allowed", allowed)
	return allowed, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
