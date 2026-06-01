// Command orders is the bar ordering microservice.
//
// It exposes the orders.v1.OrderService over gRPC:
//
//	CreateOrder       — place an order (public; QR self-order)
//	GetOrder          — fetch an order and its items (public)
//	ListOrders        — list orders, optionally by status (staff/KDS)
//	UpdateOrderStatus — advance an order's status (staff/KDS)
//
// When an order is created the service calls the catalog service to resolve
// each line's current name, price and availability, snapshots them onto the
// order item and computes the total server-side. Authorization for the
// staff-only methods is enforced at the gateway.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/dafsic/webx/app"
	ordersv1 "github.com/dafsic/webx/proto_go/orders/v1"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// Module is the orders service module.
type Module struct{}

// New returns a new orders Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return moduleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    "orders-grpc-addr",
			Value:   defaultGRPCAddr,
			EnvVars: []string{"ORDERS_GRPC_ADDR"},
			Usage:   "gRPC listen address",
		},
		&cli.StringFlag{
			Name:    "orders-catalog-addr",
			Value:   defaultCatalogAddr,
			EnvVars: []string{"ORDERS_CATALOG_ADDR"},
			Usage:   "catalog service gRPC endpoint",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cfg := &Config{
		GRPCAddr:    ctx.String("orders-grpc-addr"),
		CatalogAddr: ctx.String("orders-catalog-addr"),
	}

	return fx.Options(
		fx.Supply(cfg),
		fx.Provide(
			NewRepository,
			provideCatalogClient,
			NewServer,
		),
		fx.Invoke(runGRPCServer),
	)
}

// provideCatalogClient builds the catalog gRPC client from configuration.
func provideCatalogClient(cfg *Config) (*catalogClient, error) {
	return newCatalogClient(cfg.CatalogAddr)
}

// runGRPCServer builds the gRPC server, registers the service and wires its
// lifecycle (including the catalog client) into fx.
func runGRPCServer(
	lc fx.Lifecycle,
	cfg *Config,
	svc ordersv1.OrderServiceServer,
	catalog *catalogClient,
	logger *slog.Logger,
) error {
	srv := grpc.NewServer()
	ordersv1.RegisterOrderServiceServer(srv, svc)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ln, err := net.Listen("tcp", cfg.GRPCAddr)
			if err != nil {
				return fmt.Errorf("orders: listen %s: %w", cfg.GRPCAddr, err)
			}
			go func() {
				logger.Info("orders: gRPC server listening", "addr", cfg.GRPCAddr)
				if err := srv.Serve(ln); err != nil {
					logger.Error("orders: gRPC server stopped", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			srv.GracefulStop()
			return catalog.Close()
		},
	})
	return nil
}
