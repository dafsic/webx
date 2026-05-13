// Package otel wires the OpenTelemetry SDK (TracerProvider) into the webx
// framework. It registers a global TracerProvider with an OTLP/gRPC exporter
// that targets a local Alloy collector by default. Metrics are intentionally
// left out for now; they will be added when the platform onboards Prometheus.
//
// CLI flags (all also via env, prefix OTEL_*):
//
//	--otel-endpoint        host:port of the OTLP gRPC collector (alloy:4317)
//	--otel-service-name    resource service.name (defaults to cli app name)
//	--otel-sample-ratio    [0,1]   default 1.0
//	--otel-insecure        bool    default true (internal network plaintext)
//	--otel-disabled        bool    default false (true installs a no-op TP)
//
// On startup the exporter is created lazily; failures are logged but do not
// block the application — observability must not take production down. On
// shutdown, the provider is flushed and shut down with a 5s deadline.
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/dafsic/webx/app"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/fx"
)

// Module is the otel module.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{Name: FlagEndpoint, Value: DefaultEndpoint, EnvVars: []string{EnvEndpoint}, Usage: "OTLP gRPC endpoint (host:port)"},
		&cli.StringFlag{Name: FlagServiceName, EnvVars: []string{EnvServiceName}, Usage: "service.name resource attribute (defaults to app name)"},
		&cli.Float64Flag{Name: FlagSampleRatio, Value: DefaultSampleRatio, EnvVars: []string{EnvSampleRatio}, Usage: "Trace sample ratio in [0,1]"},
		&cli.BoolFlag{Name: FlagInsecure, Value: true, EnvVars: []string{EnvInsecure}, Usage: "Use plaintext gRPC for OTLP"},
		&cli.BoolFlag{Name: FlagDisabled, EnvVars: []string{EnvDisabled}, Usage: "Disable the OTel SDK entirely (no-op tracer)"},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	svcName := ctx.String(FlagServiceName)
	if svcName == "" {
		svcName = os.Getenv("APP_NAME")
	}

	base := []Option{
		WithEndpoint(ctx.String(FlagEndpoint)),
		WithSampleRatio(ctx.Float64(FlagSampleRatio)),
		WithInsecure(ctx.Bool(FlagInsecure)),
		WithDisabled(ctx.Bool(FlagDisabled)),
	}
	if svcName != "" {
		base = append(base, WithServiceName(svcName))
	}

	return fx.Options(
		fx.Provide(
			func(extras []Option, logger *slog.Logger) (*Config, error) {
				cfg := defaultConfig()
				if cfg.ServiceName == "" {
					cfg.ServiceName = "webx"
				}
				for _, o := range append(base, extras...) {
					if err := o(cfg); err != nil {
						return nil, err
					}
				}
				logger.Info("otel: config",
					"endpoint", cfg.Endpoint,
					"service", cfg.ServiceName,
					"sample", cfg.SampleRatio,
					"insecure", cfg.Insecure,
					"disabled", cfg.Disabled,
				)
				return cfg, nil
			},
			fx.Annotate(
				func(extras []Option) []Option { return extras },
				fx.ParamTags(`group:"`+OptionsGroup+`"`),
			),
			newTracerProvider,
		),
		fx.Invoke(func(tp trace.TracerProvider) {
			otel.SetTracerProvider(tp)
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{}, propagation.Baggage{},
			))
		}),
	)
}

// newTracerProvider builds the SDK TracerProvider (or no-op when disabled).
// It is annotated so fx can hook OnStop.
func newTracerProvider(lc fx.Lifecycle, cfg *Config, logger *slog.Logger) (trace.TracerProvider, error) {
	if cfg.Disabled {
		logger.Info("otel: disabled, installing no-op tracer provider")
		return noop.NewTracerProvider(), nil
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptrace.New(context.Background(), otlptracegrpc.NewClient(opts...))
	if err != nil {
		// Do not fail startup just because the collector is unreachable.
		logger.Warn("otel: exporter init failed; falling back to no-op", "err", err)
		return noop.NewTracerProvider(), nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		sdktrace.WithResource(res),
	)

	lc.Append(fx.Hook{
		OnStop: func(c context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(c, 5*time.Second)
			defer cancel()
			if err := tp.Shutdown(shutdownCtx); err != nil {
				logger.Warn("otel: shutdown error", "err", err)
			}
			return nil
		},
	})
	return tp, nil
}
