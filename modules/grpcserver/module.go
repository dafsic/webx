// Package grpcserver is a reusable webx module that creates and runs a
// gRPC *Server, wired with OTel tracing, panic recovery and slog logging
// interceptors. Health checks and server reflection are enabled by default.
//
// Business modules consume the provided *grpc.Server in their fx.Invoke and
// call <pkg>.RegisterXxxServer(s, impl). The lifecycle hook installed by
// this module starts the listener AFTER all such registrations, so order of
// installation does not matter for correctness.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"time"

	"github.com/dafsic/webx/app"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Module is the gRPC server module.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{Name: FlagAddr, Value: DefaultAddr, EnvVars: []string{EnvAddr}, Usage: "gRPC listen address"},
		&cli.BoolFlag{Name: FlagReflection, Value: DefaultReflection, EnvVars: []string{EnvReflection}, Usage: "Enable gRPC server reflection"},
		&cli.BoolFlag{Name: FlagHealth, Value: DefaultHealth, EnvVars: []string{EnvHealth}, Usage: "Enable grpc.health.v1 service"},
		&cli.DurationFlag{Name: FlagStopTimeout, Value: 10 * time.Second, EnvVars: []string{EnvStopTimeout}, Usage: "GracefulStop timeout"},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	base := []Option{
		WithAddr(ctx.String(FlagAddr)),
		WithReflection(ctx.Bool(FlagReflection)),
		WithHealth(ctx.Bool(FlagHealth)),
		WithStopTimeout(ctx.Duration(FlagStopTimeout)),
	}

	return fx.Options(
		fx.Provide(
			fx.Annotate(
				func(extras []Option) []Option { return extras },
				fx.ParamTags(`group:"`+OptionsGroup+`"`),
			),
			func(extras []Option) (*Config, error) {
				cfg := defaultConfig()
				for _, o := range append(base, extras...) {
					if err := o(cfg); err != nil {
						return nil, err
					}
				}
				return cfg, nil
			},
			newServer,
			func() *health.Server { return health.NewServer() },
		),
		fx.Invoke(registerStandard, runServer),
	)
}

// newServer constructs the *grpc.Server with the standard interceptors.
func newServer(cfg *Config, logger *slog.Logger) *grpc.Server {
	opts := append([]grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			recoveryUnary(logger),
			loggingUnary(logger),
		),
		grpc.ChainStreamInterceptor(
			recoveryStream(logger),
		),
	}, cfg.ServerOpts...)
	return grpc.NewServer(opts...)
}

// registerStandard registers health + reflection if enabled.
func registerStandard(cfg *Config, s *grpc.Server, hs *health.Server) {
	if cfg.Health {
		healthpb.RegisterHealthServer(s, hs)
		hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	}
	if cfg.Reflection {
		reflection.Register(s)
	}
}

// runServer hooks the listener / Serve / GracefulStop into the fx lifecycle.
func runServer(lc fx.Lifecycle, cfg *Config, s *grpc.Server, logger *slog.Logger) {
	var lis net.Listener
	servErr := make(chan error, 1)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			l, err := net.Listen("tcp", cfg.Addr)
			if err != nil {
				return fmt.Errorf("grpcserver: listen %s: %w", cfg.Addr, err)
			}
			lis = l
			logger.Info("grpcserver: listening", "addr", cfg.Addr)
			go func() { servErr <- s.Serve(lis) }()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			done := make(chan struct{})
			go func() { s.GracefulStop(); close(done) }()
			t := time.NewTimer(cfg.StopTimeout)
			defer t.Stop()
			select {
			case <-done:
				logger.Info("grpcserver: graceful stop done")
			case <-t.C:
				logger.Warn("grpcserver: graceful stop timed out, forcing stop")
				s.Stop()
			case <-stopCtx.Done():
				s.Stop()
			}
			if err := <-servErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				return err
			}
			return nil
		},
	})
}

// loggingUnary records gRPC call outcomes at debug/warn levels.
func loggingUnary(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)
		if err != nil {
			code := status.Code(err)
			lvl := slog.LevelWarn
			if code == codes.OK {
				lvl = slog.LevelDebug
			}
			logger.Log(ctx, lvl, "grpc call failed",
				"method", info.FullMethod, "code", code.String(), "dur", dur, "err", err)
			return nil, err
		}
		logger.Debug("grpc call ok", "method", info.FullMethod, "dur", dur)
		return resp, nil
	}
}

// recoveryUnary turns panics into Internal status errors.
func recoveryUnary(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				logger.Error("grpc panic", "method", info.FullMethod, "panic", p, "stack", string(debug.Stack()))
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func recoveryStream(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if p := recover(); p != nil {
				logger.Error("grpc stream panic", "method", info.FullMethod, "panic", p, "stack", string(debug.Stack()))
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(srv, ss)
	}
}
