package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/internal/people/dao"
	"github.com/dafsic/webx/internal/people/model"
	"github.com/dafsic/webx/pkg/authmd"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"
)

// Flag identifiers for the people-api module.
const (
	ModuleName = "people-api"

	FlagJWTSecret   = "jwt-secret"
	FlagJWTTTL      = "jwt-ttl"
	FlagJWTIssuer   = "jwt-issuer"
	FlagTokenPrefix = "redis-token-prefix"
	FlagSeedUser    = "people-seed-username"
	FlagSeedPass    = "people-seed-password"

	EnvJWTSecret   = "JWT_SECRET"
	EnvJWTTTL      = "JWT_TTL"
	EnvJWTIssuer   = "JWT_ISSUER"
	EnvTokenPrefix = "REDIS_TOKEN_PREFIX"
	EnvSeedUser    = "PEOPLE_SEED_USERNAME"
	EnvSeedPass    = "PEOPLE_SEED_PASSWORD"

	DefaultJWTTTL      = 24 * time.Hour
	DefaultJWTIssuer   = "webx-people"
	DefaultTokenPrefix = "people:token:"
	DefaultSeedUser    = "admin"
)

// Module is the people-api module.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{Name: FlagJWTSecret, EnvVars: []string{EnvJWTSecret}, Usage: "HS256 JWT secret (required)"},
		&cli.DurationFlag{Name: FlagJWTTTL, Value: DefaultJWTTTL, EnvVars: []string{EnvJWTTTL}, Usage: "JWT lifetime"},
		&cli.StringFlag{Name: FlagJWTIssuer, Value: DefaultJWTIssuer, EnvVars: []string{EnvJWTIssuer}, Usage: "JWT issuer claim"},
		&cli.StringFlag{Name: FlagTokenPrefix, Value: DefaultTokenPrefix, EnvVars: []string{EnvTokenPrefix}, Usage: "Redis key prefix for tokens"},
		&cli.StringFlag{Name: FlagSeedUser, Value: DefaultSeedUser, EnvVars: []string{EnvSeedUser}, Usage: "Seed admin username"},
		&cli.StringFlag{Name: FlagSeedPass, EnvVars: []string{EnvSeedPass}, Usage: "Seed admin password (empty disables)"},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	secret := ctx.String(FlagJWTSecret)
	ttl := ctx.Duration(FlagJWTTTL)
	issuer := ctx.String(FlagJWTIssuer)
	tokenPrefix := ctx.String(FlagTokenPrefix)
	seedUser := ctx.String(FlagSeedUser)
	seedPass := ctx.String(FlagSeedPass)

	return fx.Options(
		fx.Provide(
			dao.NewUserDAO,
			func() (*authmd.Signer, error) { return authmd.New(secret, ttl, issuer) },
		),
		fx.Provide(func(users dao.UserDAO, signer *authmd.Signer, rdb redisClient, l *slog.Logger) *LoginService {
			return NewLoginService(LoginParams{
				Users: users, Signer: signer, Redis: rdb,
				TokenPrefix: tokenPrefix, Logger: l,
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, users dao.UserDAO, l *slog.Logger) {
			if seedPass == "" {
				return
			}
			lc.Append(fx.Hook{
				OnStart: func(c context.Context) error {
					return seedAdmin(c, users, l, seedUser, seedPass)
				},
			})
		}),
	)
}

func seedAdmin(ctx context.Context, users dao.UserDAO, logger *slog.Logger, username, password string) error {
	_, err := users.GetByUsername(ctx, username)
	if err == nil {
		logger.Info("people: seed user already exists", "username", username)
		return nil
	}
	if !errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("people: seed lookup: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("people: seed hash: %w", err)
	}
	id, err := users.Create(ctx, &model.User{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     username,
	})
	if err != nil {
		return fmt.Errorf("people: seed create: %w", err)
	}
	logger.Info("people: seed user created", "username", username, "id", id)
	return nil
}
