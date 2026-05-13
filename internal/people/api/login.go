// Package api implements the business logic of the people service. Layers
// above (gRPC adapter in internal/people/server) only translate types; auth
// token signing lives in pkg/authmd; persistence in internal/people/dao.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dafsic/webx/internal/people/dao"
	"github.com/dafsic/webx/pkg/authmd"
	"github.com/dafsic/webx/pkg/grpcerr"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// LoginService is the use case behind PeopleService.Login.
type LoginService struct {
	users       dao.UserDAO
	signer      *authmd.Signer
	rdb         redis.UniversalClient
	tokenPrefix string
	logger      *slog.Logger
}

// LoginParams bundles dependencies (used by NewLoginService).
type LoginParams struct {
	Users       dao.UserDAO
	Signer      *authmd.Signer
	Redis       redis.UniversalClient
	TokenPrefix string
	Logger      *slog.Logger
}

// NewLoginService constructs a LoginService.
func NewLoginService(p LoginParams) *LoginService {
	if p.TokenPrefix == "" {
		p.TokenPrefix = "people:token:"
	}
	return &LoginService{
		users:       p.Users,
		signer:      p.Signer,
		rdb:         p.Redis,
		tokenPrefix: p.TokenPrefix,
		logger:      p.Logger,
	}
}

// LoginResult is the use-case-level output (decoupled from proto types).
type LoginResult struct {
	Token     string
	JTI       string
	ExpiresAt time.Time
	UserID    int64
	Username  string
	Nickname  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Login authenticates by username+password, mints a JWT and stores its jti
// in Redis with the same TTL as the token. Returns grpcerr.* sentinel errors
// for failure modes.
func (s *LoginService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("%w: username/password required", grpcerr.ErrInvalidArgument)
	}

	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: invalid credentials", grpcerr.ErrUnauthenticated)
		}
		return nil, fmt.Errorf("people: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("%w: invalid credentials", grpcerr.ErrUnauthenticated)
	}

	token, jti, expiresAt, err := s.signer.Issue(user.ID)
	if err != nil {
		return nil, fmt.Errorf("people: issue token: %w", err)
	}
	key := s.tokenPrefix + jti
	if err := s.rdb.Set(ctx, key, user.ID, time.Until(expiresAt)).Err(); err != nil {
		return nil, fmt.Errorf("people: persist token: %w", err)
	}

	s.logger.Info("people: login ok",
		"user_id", user.ID, "username", user.Username, "jti", jti,
		"expires_at", expiresAt.Unix())

	return &LoginResult{
		Token:     token,
		JTI:       jti,
		ExpiresAt: expiresAt,
		UserID:    user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}
