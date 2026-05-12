// Package redis provides a go-redis/v9 based redis module for the webx
// framework.
//
// CLI flags registered (all with matching REDIS_* environment variables):
//
//	--redis-addrs          comma-separated list (default 127.0.0.1:6379)
//	--redis-db             integer DB index (default 0)
//	--redis-username       ACL username
//	--redis-password       password
//	--redis-master-name    sentinel master name (enables failover mode)
//	--redis-pool-size      connection pool size (default 10)
//	--redis-dial-timeout   dial timeout         (default 5s)
//	--redis-read-timeout   read timeout         (default 3s)
//	--redis-write-timeout  write timeout        (default 3s)
//
// Provided into the fx graph:
//
//	redis.UniversalClient — primary client; cluster / sentinel / single-node
//	                        selection is delegated to go-redis based on the
//	                        resolved Config.
//	*redis.Config         — resolved configuration.
//
// The client is pinged during fx OnStart (subject to PingTimeout) and Close()
// is called during OnStop.
//
// Extending: callers can contribute extra Options via the
// group:"redis_options" fx group. For example, to force TLS-aware settings
// from another module:
//
//	fx.Supply(fx.Annotate(
//	    redis.WithPoolSize(64),
//	    fx.ResultTags(`group:"`+redis.OptionsGroup+`"`),
//	))
package redis

import (
	"context"
	"fmt"

	"github.com/dafsic/webx/app"
	goredis "github.com/redis/go-redis/v9"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

// Module is the redis module.
type Module struct{}

// New returns a new redis Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    FlagAddrs,
			Value:   DefaultAddr,
			EnvVars: []string{EnvAddrs},
			Usage:   "Redis address(es), comma-separated for cluster/sentinel",
		},
		&cli.IntFlag{
			Name:    FlagDB,
			Value:   DefaultDB,
			EnvVars: []string{EnvDB},
			Usage:   "Redis database index (single-node only)",
		},
		&cli.StringFlag{
			Name:    FlagUsername,
			EnvVars: []string{EnvUsername},
			Usage:   "Redis ACL username (Redis 6+)",
		},
		&cli.StringFlag{
			Name:    FlagPassword,
			EnvVars: []string{EnvPassword},
			Usage:   "Redis password",
		},
		&cli.StringFlag{
			Name:    FlagMasterName,
			EnvVars: []string{EnvMasterName},
			Usage:   "Sentinel master name (enables failover mode if set)",
		},
		&cli.IntFlag{
			Name:    FlagPoolSize,
			Value:   DefaultPoolSize,
			EnvVars: []string{EnvPoolSize},
			Usage:   "Redis connection pool size",
		},
		&cli.DurationFlag{
			Name:    FlagDialTimeout,
			Value:   DefaultDialTimeout,
			EnvVars: []string{EnvDialTimeout},
			Usage:   "Redis dial timeout",
		},
		&cli.DurationFlag{
			Name:    FlagReadTimeout,
			Value:   DefaultReadTimeout,
			EnvVars: []string{EnvReadTimeout},
			Usage:   "Redis read timeout",
		},
		&cli.DurationFlag{
			Name:    FlagWriteTimeout,
			Value:   DefaultWriteTimeout,
			EnvVars: []string{EnvWriteTimeout},
			Usage:   "Redis write timeout",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cliOpts := []Option{
		WithAddrsCSV(ctx.String(FlagAddrs)),
		WithDB(ctx.Int(FlagDB)),
		WithUsername(ctx.String(FlagUsername)),
		WithPassword(ctx.String(FlagPassword)),
		WithMasterName(ctx.String(FlagMasterName)),
		WithPoolSize(ctx.Int(FlagPoolSize)),
		WithDialTimeout(ctx.Duration(FlagDialTimeout)),
		WithReadTimeout(ctx.Duration(FlagReadTimeout)),
		WithWriteTimeout(ctx.Duration(FlagWriteTimeout)),
	}

	return fx.Options(
		fx.Supply(fx.Annotate(
			cliOpts,
			fx.ResultTags(`group:"`+OptionsGroup+`,flatten"`),
		)),
		fx.Provide(fx.Annotate(
			NewConfig,
			fx.ParamTags(`group:"`+OptionsGroup+`"`),
		)),
		fx.Provide(newClient),
	)
}

func newClient(lc fx.Lifecycle, cfg *Config) (goredis.UniversalClient, error) {
	client := goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs:        cfg.Addrs,
		DB:           cfg.DB,
		Username:     cfg.Username,
		Password:     cfg.Password,
		MasterName:   cfg.MasterName,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			if cfg.PingTimeout <= 0 {
				return nil
			}
			pingCtx, cancel := context.WithTimeout(startCtx, cfg.PingTimeout)
			defer cancel()
			if err := client.Ping(pingCtx).Err(); err != nil {
				_ = client.Close()
				return fmt.Errorf("redis: ping %v: %w", cfg.Addrs, err)
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
