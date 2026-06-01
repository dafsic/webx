// Command gateway is the public HTTP entry point for the webx platform.
//
// It runs a grpc-gateway reverse proxy: browsers and external clients speak
// HTTP/JSON to this service, which translates each request into a gRPC call to
// the appropriate backend microservice (e.g. people). REST mappings and the
// OpenAPI documentation are both derived from the .proto definitions, so adding
// or changing an endpoint only requires regenerating proto_go.
//
// Endpoints:
//
//	/api/v1/...     — REST routes proxied to backend gRPC services
//	/docs           — Swagger UI
//	/openapi.json   — generated OpenAPI (swagger 2.0) document
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/modules/logs"
	"github.com/dafsic/webx/utils"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

// Module is the gateway module.
type Module struct{}

// New returns a new gateway Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return moduleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    "gateway-http-addr",
			Value:   defaultHTTPAddr,
			EnvVars: []string{"GATEWAY_HTTP_ADDR"},
			Usage:   "HTTP listen address",
		},
		&cli.StringFlag{
			Name:    "gateway-jwt-secret",
			EnvVars: []string{"GATEWAY_JWT_SECRET"},
			Usage:   "HS256 secret shared with the people service for token validation (required)",
		},
		&cli.StringFlag{
			Name:    "gateway-people-addr",
			Value:   defaultPeopleAddr,
			EnvVars: []string{"GATEWAY_PEOPLE_ADDR"},
			Usage:   "people service gRPC endpoint",
		},
		&cli.StringFlag{
			Name:    "gateway-catalog-addr",
			Value:   defaultCatalogAddr,
			EnvVars: []string{"GATEWAY_CATALOG_ADDR"},
			Usage:   "catalog service gRPC endpoint",
		},
		&cli.StringFlag{
			Name:    "gateway-orders-addr",
			Value:   defaultOrdersAddr,
			EnvVars: []string{"GATEWAY_ORDERS_ADDR"},
			Usage:   "orders service gRPC endpoint",
		},
		&cli.StringFlag{
			Name:    "gateway-openapi-spec",
			Value:   defaultOpenAPISpec,
			EnvVars: []string{"GATEWAY_OPENAPI_SPEC"},
			Usage:   "path to the generated OpenAPI swagger JSON (empty disables /docs)",
		},
		&cli.StringFlag{
			Name:    "gateway-cors-origins",
			Value:   defaultCORSOrigins,
			EnvVars: []string{"GATEWAY_CORS_ORIGINS"},
			Usage:   "comma-separated allowed CORS origins, or * for any",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cfg := &Config{
		HTTPAddr:    ctx.String("gateway-http-addr"),
		JWTSecret:   ctx.String("gateway-jwt-secret"),
		PeopleAddr:  ctx.String("gateway-people-addr"),
		CatalogAddr: ctx.String("gateway-catalog-addr"),
		OrdersAddr:  ctx.String("gateway-orders-addr"),
		OpenAPISpec: ctx.String("gateway-openapi-spec"),
		CORSOrigins: utils.StrSplit(ctx.String("gateway-cors-origins")),
	}

	return fx.Options(
		fx.Supply(cfg),
		fx.Invoke(runHTTPServer),
	)
}

// runHTTPServer builds the gateway handler and wires the HTTP server into the
// fx lifecycle.
func runHTTPServer(lc fx.Lifecycle, cfg *Config, logger *slog.Logger) error {
	if cfg.JWTSecret == "" {
		return fmt.Errorf("gateway: --gateway-jwt-secret (GATEWAY_JWT_SECRET) is required")
	}

	// A long-lived context bounds the gRPC client connections created by the
	// gateway handlers; it is cancelled on shutdown.
	ctx, cancel := context.WithCancel(context.Background())

	handler, err := buildHandler(ctx, cfg, logger)
	if err != nil {
		cancel()
		return err
	}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: handler}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				logger.Info("gateway: HTTP server listening", "addr", cfg.HTTPAddr)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("gateway: HTTP server stopped", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			cancel()
			if err := srv.Shutdown(stopCtx); err != nil {
				return fmt.Errorf("gateway: shutdown: %w", err)
			}
			return nil
		},
	})
	return nil
}

func main() {
	a := app.NewApplication("gateway", "Webx HTTP/JSON ⇆ gRPC gateway")
	a.Install(
		logs.New(),
		New(),
	)
	if err := a.Run(); err != nil {
		panic(err)
	}
}
