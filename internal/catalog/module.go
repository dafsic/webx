// Command catalog is the bar menu (items + prices) microservice.
//
// It exposes the catalog.v1.CatalogService over gRPC:
//
//	ListItems  — list the menu (public)
//	GetItem    — fetch a single item (public)
//	CreateItem — add an item (admin)
//	UpdateItem — modify an item (admin)
//	DeleteItem — remove an item (admin)
//
// Authorization for the admin methods is enforced at the gateway / by a future
// interceptor; this service focuses on the menu data itself.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/dafsic/webx/app"
	catalogv1 "github.com/dafsic/webx/proto_go/catalog/v1"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// Module is the catalog service module.
type Module struct{}

// New returns a new catalog Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return moduleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    "catalog-grpc-addr",
			Value:   defaultGRPCAddr,
			EnvVars: []string{"CATALOG_GRPC_ADDR"},
			Usage:   "gRPC listen address",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cfg := &Config{
		GRPCAddr: ctx.String("catalog-grpc-addr"),
	}

	return fx.Options(
		fx.Supply(cfg),
		fx.Provide(
			NewRepository,
			NewServer,
		),
		fx.Invoke(runGRPCServer),
	)
}

// runGRPCServer builds the gRPC server, registers the service and wires its
// lifecycle into fx.
func runGRPCServer(
	lc fx.Lifecycle,
	cfg *Config,
	svc catalogv1.CatalogServiceServer,
	logger *slog.Logger,
) error {
	srv := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(srv, svc)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ln, err := net.Listen("tcp", cfg.GRPCAddr)
			if err != nil {
				return fmt.Errorf("catalog: listen %s: %w", cfg.GRPCAddr, err)
			}
			go func() {
				logger.Info("catalog: gRPC server listening", "addr", cfg.GRPCAddr)
				if err := srv.Serve(ln); err != nil {
					logger.Error("catalog: gRPC server stopped", "err", err)
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
