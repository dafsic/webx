// Package logs provides a slog-based logging module for the webx framework.
//
// CLI flags registered:
//
//	--log-level   debug | info | warn | error | panic   (default: info)
//	--log-format  text  | json                          (default: text)
//
// Provided into the fx graph:
//
//	*slog.Logger       — primary logger
//	*slog.LevelVar     — runtime-mutable level (also drives the logger)
//	*logs.Config       — resolved configuration
//
// Side effects:
//
//	slog.SetDefault(logger) is invoked so package-level slog.Info / slog.Error
//	calls share the same configuration.
//
// Extending: any caller may contribute extra Options through the
// group:"logs_options" fx group. For example, to enable source information
// from another module:
//
//	fx.Supply(fx.Annotate(
//	    logs.WithSource(true),
//	    fx.ResultTags(`group:"`+logs.OptionsGroup+`"`),
//	))
package logs

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/dafsic/webx/app"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

// LevelPanic is a custom slog level that sits above slog.LevelError. slog has
// no native panic level, so we synthesise one to satisfy the
// debug/info/warn/error/panic contract.
const LevelPanic slog.Level = 12

// Module is the logs module.
type Module struct{}

// New returns a new logs Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    FlagLevel,
			Value:   DefaultLevel,
			EnvVars: []string{EnvLevel},
			Usage:   "Log level (debug | info | warn | error | panic)",
		},
		&cli.StringFlag{
			Name:    FlagFormat,
			Value:   DefaultFormat,
			EnvVars: []string{EnvFormat},
			Usage:   "Log format (text | json)",
		},
	)
}

// Install implements app.Module.
//
// The CLI-derived options are pushed into the same group:"logs_options" fx
// group used for external extensions, so user-supplied options always win
// (NewConfig applies them in the order fx delivers them, and later options
// overwrite earlier ones).
func (m *Module) Install(ctx app.Context) fx.Option {
	cliOpts := []Option{
		WithLevel(ctx.String(FlagLevel)),
		WithFormat(ctx.String(FlagFormat)),
	}

	return fx.Options(
		// Push the CLI-derived options into the group.
		fx.Supply(fx.Annotate(
			cliOpts,
			fx.ResultTags(`group:"`+OptionsGroup+`,flatten"`),
		)),
		// Build the Config by collecting all Options from the group.
		fx.Provide(fx.Annotate(
			NewConfig,
			fx.ParamTags(`group:"`+OptionsGroup+`"`),
		)),
		// Build the Logger from the Config.
		fx.Provide(newLogger, exposeLevelVar),
		fx.Invoke(func(l *slog.Logger) { slog.SetDefault(l) }),
	)
}

func newLogger(cfg *Config) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				if l, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(levelName(l))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(cfg.Writer, opts)
	case "text", "":
		handler = slog.NewTextHandler(cfg.Writer, opts)
	default:
		return nil, fmt.Errorf("logs: unknown format %q", cfg.Format)
	}
	return slog.New(handler), nil
}

func exposeLevelVar(cfg *Config) *slog.LevelVar { return cfg.Level }

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error", "err":
		return slog.LevelError, nil
	case "panic", "fatal":
		return LevelPanic, nil
	default:
		return 0, fmt.Errorf("logs: unknown level %q (want debug|info|warn|error|panic)", s)
	}
}

func levelName(l slog.Level) string {
	switch {
	case l >= LevelPanic:
		return "PANIC"
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

// Panic logs at the custom panic level and then panics with msg. Convenient
// for the rare "log then unwind" sites.
func Panic(logger *slog.Logger, msg string, args ...any) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Log(nil, LevelPanic, msg, args...)
	panic(msg)
}
