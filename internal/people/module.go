// Command people is the EVM-wallet authentication and RBAC microservice.
//
// It exposes the people.v1.PeopleService over gRPC:
//
//	GetChallenge    — issue a nonce for the wallet to sign (public)
//	Login           — verify an EIP-191 personal_sign signature, auto-register
//	                  first-time wallets and return a JWT session (public)
//	Logout          — revoke the caller's session token (auth required)
//	CheckPermission — RBAC lookup (auth + "permission:read" required)
//
// Authenticated methods expect an "authorization: Bearer <jwt>" metadata
// header; a unary interceptor validates the token and enforces per-method RBAC.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/dafsic/webx/app"
	peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// Module is the people service module.
type Module struct{}

// New returns a new people Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return moduleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    "people-grpc-addr",
			Value:   defaultGRPCAddr,
			EnvVars: []string{"PEOPLE_GRPC_ADDR"},
			Usage:   "gRPC listen address",
		},
		&cli.StringFlag{
			Name:    "people-jwt-secret",
			EnvVars: []string{"PEOPLE_JWT_SECRET"},
			Usage:   "HMAC secret used to sign session tokens (required)",
		},
		&cli.DurationFlag{
			Name:    "people-jwt-ttl",
			Value:   defaultJWTTTL,
			EnvVars: []string{"PEOPLE_JWT_TTL"},
			Usage:   "Session token lifetime",
		},
		&cli.DurationFlag{
			Name:    "people-nonce-ttl",
			Value:   defaultNonceTTL,
			EnvVars: []string{"PEOPLE_NONCE_TTL"},
			Usage:   "Login challenge nonce lifetime",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cfg := &Config{
		GRPCAddr:  ctx.String("people-grpc-addr"),
		JWTSecret: ctx.String("people-jwt-secret"),
		JWTTTL:    ctx.Duration("people-jwt-ttl"),
		NonceTTL:  ctx.Duration("people-nonce-ttl"),
	}

	return fx.Options(
		fx.Supply(cfg),
		fx.Provide(
			NewAuthenticator,
			NewRepository,
			NewServer,
			NewAuthInterceptor,
		),
		fx.Invoke(runGRPCServer),
	)
}

// runGRPCServer builds the gRPC server, registers the service and wires its
// lifecycle into fx.
func runGRPCServer(
	lc fx.Lifecycle,
	cfg *Config,
	svc peoplev1.PeopleServiceServer,
	interceptor grpc.UnaryServerInterceptor,
	logger *slog.Logger,
) error {
	if cfg.JWTSecret == "" {
		return fmt.Errorf("people: --people-jwt-secret (PEOPLE_JWT_SECRET) is required")
	}

	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor))
	peoplev1.RegisterPeopleServiceServer(srv, svc)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ln, err := net.Listen("tcp", cfg.GRPCAddr)
			if err != nil {
				return fmt.Errorf("people: listen %s: %w", cfg.GRPCAddr, err)
			}
			go func() {
				logger.Info("people: gRPC server listening", "addr", cfg.GRPCAddr)
				if err := srv.Serve(ln); err != nil {
					logger.Error("people: gRPC server stopped", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			srv.GracefulStop()
			return nil
		},
	})
	return nil
}
