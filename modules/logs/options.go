package logs

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config is the resolved configuration used by the module to build the
// underlying slog.Handler. Options mutate this struct.
type Config struct {
	// Level controls the minimum level. Defaults to slog.LevelInfo. It is
	// exposed as a *slog.LevelVar so callers can change the level at runtime
	// (for example via an HTTP endpoint).
	Level *slog.LevelVar

	// Format selects the slog handler. "text" or "json".
	Format string

	// Writer is the destination of the log records. Defaults to os.Stdout.
	Writer io.Writer

	// AddSource toggles slog.HandlerOptions.AddSource.
	AddSource bool
}

// Option mutates a Config.
type Option func(*Config) error

// NewConfig builds a Config seeded with sane defaults and then applies the
// supplied options in order.
func NewConfig(opts ...Option) (*Config, error) {
	cfg := &Config{
		Level:     new(slog.LevelVar), // zero value == slog.LevelInfo
		Format:    DefaultFormat,
		Writer:    os.Stdout,
		AddSource: false,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// WithLevel sets the minimum log level from a string. Accepted values are
// debug | info | warn | error | panic (case-insensitive). An empty string is
// treated as the default level.
func WithLevel(level string) Option {
	return func(c *Config) error {
		lvl, err := parseLevel(level)
		if err != nil {
			return err
		}
		c.Level.Set(lvl)
		return nil
	}
}

// WithLevelVar replaces the level variable. Useful when the caller wants to
// share a single *slog.LevelVar with other components (e.g. for runtime
// level changes).
func WithLevelVar(v *slog.LevelVar) Option {
	return func(c *Config) error {
		if v == nil {
			return fmt.Errorf("logs: WithLevelVar(nil)")
		}
		c.Level = v
		return nil
	}
}

// WithFormat selects the output format: "text" or "json".
func WithFormat(format string) Option {
	return func(c *Config) error {
		f := strings.ToLower(strings.TrimSpace(format))
		switch f {
		case "":
			c.Format = DefaultFormat
		case "text", "json":
			c.Format = f
		default:
			return fmt.Errorf("logs: unknown format %q (want text|json)", format)
		}
		return nil
	}
}

// WithWriter sets the destination writer.
func WithWriter(w io.Writer) Option {
	return func(c *Config) error {
		if w == nil {
			return fmt.Errorf("logs: WithWriter(nil)")
		}
		c.Writer = w
		return nil
	}
}

// WithSource toggles inclusion of source file/line in records.
func WithSource(enabled bool) Option {
	return func(c *Config) error {
		c.AddSource = enabled
		return nil
	}
}
