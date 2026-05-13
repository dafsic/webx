package postgres

import (
	"fmt"
	"strings"
	"time"
)

// Config is the resolved configuration consumed by the module.
type Config struct {
	DSN string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// PingTimeout bounds the connectivity check performed during fx OnStart.
	// Zero disables the ping.
	PingTimeout time.Duration

	// Migrate controls automatic migration on startup. Allowed values:
	// MigrateOff (default), MigrateUp, MigrateDown.
	Migrate       string
	MigrateDir    string
	MigrateTable  string
	MigrateSchema string
}

// Option mutates a Config.
type Option func(*Config) error

// NewConfig builds a Config with framework defaults and applies the supplied
// options in order. Later options overwrite earlier ones.
func NewConfig(opts ...Option) (*Config, error) {
	cfg := &Config{
		DSN:             DefaultDSN,
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
		PingTimeout:     DefaultPingTimeout,
		Migrate:         DefaultMigrate,
		MigrateDir:      DefaultMigrateDir,
		MigrateTable:    DefaultMigrateTable,
		MigrateSchema:   DefaultMigrateSchema,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("postgres: DSN is required")
	}
	return cfg, nil
}

// WithDSN sets the DSN string.
func WithDSN(dsn string) Option {
	return func(c *Config) error {
		c.DSN = dsn
		return nil
	}
}

// WithMaxOpenConns caps the number of open connections.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) error {
		if n < 0 {
			return fmt.Errorf("postgres: WithMaxOpenConns(%d) must be >= 0", n)
		}
		c.MaxOpenConns = n
		return nil
	}
}

// WithMaxIdleConns caps the size of the idle connection pool.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) error {
		if n < 0 {
			return fmt.Errorf("postgres: WithMaxIdleConns(%d) must be >= 0", n)
		}
		c.MaxIdleConns = n
		return nil
	}
}

// WithConnMaxLifetime sets database/sql ConnMaxLifetime.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) error { c.ConnMaxLifetime = d; return nil }
}

// WithConnMaxIdleTime sets database/sql ConnMaxIdleTime.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *Config) error { c.ConnMaxIdleTime = d; return nil }
}

// WithPingTimeout bounds the startup ping. Use 0 to disable the check.
func WithPingTimeout(d time.Duration) Option {
	return func(c *Config) error { c.PingTimeout = d; return nil }
}

// WithMigrate sets the startup migration mode: off | up | down.
// An empty string is treated as off.
func WithMigrate(mode string) Option {
	return func(c *Config) error {
		m := strings.ToLower(strings.TrimSpace(mode))
		switch m {
		case "", MigrateOff:
			c.Migrate = MigrateOff
		case MigrateUp, MigrateDown:
			c.Migrate = m
		default:
			return fmt.Errorf("postgres: WithMigrate %q (want off|up|down)", mode)
		}
		return nil
	}
}

// WithMigrateDir sets the directory holding sql-migrate files.
func WithMigrateDir(dir string) Option {
	return func(c *Config) error {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return fmt.Errorf("postgres: WithMigrateDir requires a non-empty path")
		}
		c.MigrateDir = dir
		return nil
	}
}

// WithMigrateTable sets the name of the migration tracking table.
func WithMigrateTable(name string) Option {
	return func(c *Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("postgres: WithMigrateTable requires a non-empty name")
		}
		c.MigrateTable = name
		return nil
	}
}

// WithMigrateSchema sets the schema of the migration tracking table.
func WithMigrateSchema(name string) Option {
	return func(c *Config) error {
		c.MigrateSchema = strings.TrimSpace(name)
		return nil
	}
}
