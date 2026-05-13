// Package ginserver wires a gin.Engine + http.Server into the webx
// framework. It installs the standard middleware (recovery, request id,
// otelgin) and exposes the engine via fx so business modules can mount
// routes (or grpc-gateway mux) on it.
package ginserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/dafsic/webx/app"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/fx"
)

// Module is the gin HTTP server module.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{Name: FlagAddr, Value: DefaultAddr, EnvVars: []string{EnvAddr}, Usage: "HTTP listen address"},
		&cli.StringFlag{Name: FlagMode, Value: DefaultMode, EnvVars: []string{EnvMode}, Usage: "gin mode: debug|release|test"},
		&cli.DurationFlag{Name: FlagStopTimeout, Value: 10 * time.Second, EnvVars: []string{EnvStopTimeout}, Usage: "HTTP shutdown timeout"},
		&cli.StringSliceFlag{Name: FlagTrustedProxies, EnvVars: []string{EnvTrustedProxies}, Usage: "Trusted upstream proxy CIDRs (repeat or comma-separated)"},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	base := []Option{
		WithAddr(ctx.String(FlagAddr)),
		WithMode(ctx.String(FlagMode)),
		WithStopTimeout(ctx.Duration(FlagStopTimeout)),
		WithTrustedProxies(ctx.StringSlice(FlagTrustedProxies)),
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
			newEngine,
		),
		fx.Invoke(runServer),
	)
}

func newEngine(cfg *Config, logger *slog.Logger) *gin.Engine {
	gin.SetMode(cfg.Mode)
	r := gin.New()
	if cfg.TrustedProxies != nil {
		_ = r.SetTrustedProxies(cfg.TrustedProxies)
	}
	r.Use(
		requestID(),
		recovery(logger),
		otelgin.Middleware("gateway"),
		accessLog(logger),
	)
	return r
}

func runServer(lc fx.Lifecycle, cfg *Config, r *gin.Engine, logger *slog.Logger) {
	srv := &http.Server{Addr: cfg.Addr, Handler: r}
	servErr := make(chan error, 1)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			logger.Info("ginserver: listening", "addr", cfg.Addr)
			go func() {
				err := srv.ListenAndServe()
				if !errors.Is(err, http.ErrServerClosed) {
					servErr <- err
					return
				}
				servErr <- nil
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			ctx, cancel := context.WithTimeout(stopCtx, cfg.StopTimeout)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				logger.Warn("ginserver: shutdown error", "err", err)
				return err
			}
			return <-servErr
		},
	})
}

// --- middlewares ---

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		c.Set("request_id", id)
		c.Writer.Header().Set("X-Request-Id", id)
		c.Next()
	}
}

func recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				logger.Error("http panic", "path", c.Request.URL.Path,
					"panic", p, "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					gin.H{"code": "internal", "message": "internal error"})
			}
		}()
		c.Next()
	}
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)
		lvl := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			lvl = slog.LevelWarn
		}
		logger.Log(c.Request.Context(), lvl, "http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur", dur,
			"request_id", c.GetString("request_id"),
		)
	}
}
