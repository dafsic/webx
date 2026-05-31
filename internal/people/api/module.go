package api

import (
	"log/slog"
	"time"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/internal/people/dao"
	"github.com/dafsic/webx/pkg/authmd"
	"github.com/dafsic/webx/pkg/eip712"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

// Flag identifiers for the people-api module.
const (
ModuleName = "people-api"

DefaultJWTTTL          = 24 * time.Hour
DefaultJWTIssuer       = "webx-people"
DefaultTokenPrefix     = "people:token:"
DefaultChallengePrefix = "people:nonce:"
DefaultDomainName      = "WebX People"
DefaultDomainVersion   = "1"
DefaultChainID         = uint64(1) // Ethereum mainnet; override with --evm-chain-id
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
&cli.StringFlag{Name: "jwt-secret", Usage: "HS256 JWT secret (required)"},
		&cli.DurationFlag{Name: "jwt-ttl", Value: DefaultJWTTTL, Usage: "JWT lifetime"},
		&cli.StringFlag{Name: "jwt-issuer", Value: DefaultJWTIssuer, Usage: "JWT issuer claim"},
		&cli.StringFlag{Name: "redis-token-prefix", Value: DefaultTokenPrefix, Usage: "Redis key prefix for JWT sessions"},
		&cli.StringFlag{Name: "redis-challenge-prefix", Value: DefaultChallengePrefix, Usage: "Redis key prefix for EIP-712 nonces"},
		&cli.StringFlag{Name: "eip712-domain-name", Value: DefaultDomainName, Usage: "EIP-712 domain name"},
		&cli.StringFlag{Name: "eip712-domain-version", Value: DefaultDomainVersion, Usage: "EIP-712 domain version"},
		&cli.Int64Flag{Name: "evm-chain-id", Value: int64(DefaultChainID), Usage: "EVM chain ID for EIP-712 domain separator"},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	secret := ctx.String("jwt-secret")
	ttl := ctx.Duration("jwt-ttl")
	issuer := ctx.String("jwt-issuer")
	tokenPrefix := ctx.String("redis-token-prefix")
	challengePrefix := ctx.String("redis-challenge-prefix")
	domain := eip712.Domain{
		Name:    ctx.String("eip712-domain-name"),
		Version: ctx.String("eip712-domain-version"),
		ChainID: uint64(ctx.Int64("evm-chain-id")),
	}

	return fx.Options(
fx.Provide(
dao.NewUserDAO,
dao.NewRoleDAO,
func() (*authmd.Signer, error) { return authmd.New(secret, ttl, issuer) },
		),
		fx.Provide(func(users dao.UserDAO, roles dao.RoleDAO, signer *authmd.Signer, rdb redisClient, l *slog.Logger) *Service {
return NewService(Params{
Users:           users,
Roles:           roles,
Signer:          signer,
Redis:           rdb,
Domain:          domain,
TokenPrefix:     tokenPrefix,
ChallengePrefix: challengePrefix,
Logger:          l,
})
}),
	)
}
