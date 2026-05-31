// Package grpcclient exposes a *Dialer factory for outbound gRPC
// connections. Each downstream microservice has its own gateway-side flag
// (e.g. --people-grpc-addr); the gateway module reads the address and asks
// the Dialer for a *grpc.ClientConn.
//
// All dialed connections share:
//   - otelgrpc stats handler for trace propagation,
//   - keepalive pings,
//   - insecure transport (internal network — wrap with WithTransportCredentials
//     in Options for cross-trust-boundary calls).
package grpcclient

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dafsic/webx/app"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Module is the gRPC client factory module.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(*cli.App) {}

// Install implements app.Module.
func (m *Module) Install(_ app.Context) fx.Option {
	return fx.Options(
		fx.Provide(
			fx.Annotate(
				func(extras []Option) []Option { return extras },
				fx.ParamTags(`group:"grpcclient_options"`),
			),
			func(extras []Option) (*Config, error) {
				cfg := defaultConfig()
				for _, o := range extras {
					if err := o(cfg); err != nil {
						return nil, err
					}
				}
				return cfg, nil
			},
			newDialer,
		),
	)
}

// Dialer creates outbound gRPC connections with the shared defaults.
type Dialer struct {
	cfg    *Config
	logger *slog.Logger
}

func newDialer(cfg *Config, logger *slog.Logger) *Dialer {
	return &Dialer{cfg: cfg, logger: logger}
}

// Dial connects to target (host:port) and returns a *grpc.ClientConn.
func (d *Dialer) Dial(ctx context.Context, target string, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	if target == "" {
		return nil, fmt.Errorf("grpcclient: empty target")
	}
	dialCtx, cancel := context.WithTimeout(ctx, d.cfg.DialTimeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                d.cfg.KeepaliveTime,
			Timeout:             10 * 1e9, // 10s
			PermitWithoutStream: true,
		}),
	}
	opts = append(opts, d.cfg.ExtraDialOpts...)
	opts = append(opts, extra...)

	d.logger.Info("grpcclient: dialing", "target", target)
	cc, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial %s: %w", target, err)
	}
	_ = dialCtx
	return cc, nil
}
