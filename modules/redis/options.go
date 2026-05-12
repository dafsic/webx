package redis

import (
	"fmt"
	"strings"
	"time"
)

// Config is the resolved configuration consumed by the module to construct
// a *redis.UniversalClient. Options mutate this struct.
//
// The fields mirror a useful subset of redis.UniversalOptions. When a single
// address is provided a regular client is built; when multiple addresses are
// provided a cluster client is built; when MasterName is non-empty a
// sentinel/failover client is built. This selection is performed by
// go-redis itself through NewUniversalClient.
type Config struct {
	Addrs      []string
	DB         int
	Username   string
	Password   string
	MasterName string

	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// PingTimeout bounds the connectivity check performed during fx OnStart.
	// Zero disables the ping.
	PingTimeout time.Duration
}

// Option mutates a Config.
type Option func(*Config) error

// NewConfig builds a Config with framework defaults and applies the supplied
// options in order. Later options overwrite earlier ones.
func NewConfig(opts ...Option) (*Config, error) {
	cfg := &Config{
		Addrs:        []string{DefaultAddr},
		DB:           DefaultDB,
		PoolSize:     DefaultPoolSize,
		DialTimeout:  DefaultDialTimeout,
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		PingTimeout:  DefaultPingTimeout,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	if len(cfg.Addrs) == 0 {
		return nil, fmt.Errorf("redis: at least one address is required")
	}
	return cfg, nil
}

// WithAddrs sets the server addresses. The csv variant accepts a single
// comma-separated string, which is the form delivered by the CLI flag.
func WithAddrs(addrs ...string) Option {
	return func(c *Config) error {
		cleaned := cleanList(addrs)
		if len(cleaned) == 0 {
			return fmt.Errorf("redis: WithAddrs requires at least one non-empty address")
		}
		c.Addrs = cleaned
		return nil
	}
}

// WithAddrsCSV parses a comma-separated address list. Empty string keeps
// the current value (so users can leave the flag unset).
func WithAddrsCSV(csv string) Option {
	return func(c *Config) error {
		csv = strings.TrimSpace(csv)
		if csv == "" {
			return nil
		}
		parts := strings.Split(csv, ",")
		cleaned := cleanList(parts)
		if len(cleaned) == 0 {
			return fmt.Errorf("redis: WithAddrsCSV produced no usable address from %q", csv)
		}
		c.Addrs = cleaned
		return nil
	}
}

// WithDB sets the default database index. Ignored by cluster clients.
func WithDB(db int) Option {
	return func(c *Config) error {
		if db < 0 {
			return fmt.Errorf("redis: WithDB(%d) must be >= 0", db)
		}
		c.DB = db
		return nil
	}
}

// WithUsername sets the ACL username (Redis 6+).
func WithUsername(u string) Option {
	return func(c *Config) error { c.Username = u; return nil }
}

// WithPassword sets the authentication password.
func WithPassword(p string) Option {
	return func(c *Config) error { c.Password = p; return nil }
}

// WithMasterName enables sentinel/failover mode by setting the master name.
func WithMasterName(name string) Option {
	return func(c *Config) error { c.MasterName = strings.TrimSpace(name); return nil }
}

// WithPoolSize sets the connection pool size. Zero keeps the go-redis
// default.
func WithPoolSize(n int) Option {
	return func(c *Config) error {
		if n < 0 {
			return fmt.Errorf("redis: WithPoolSize(%d) must be >= 0", n)
		}
		c.PoolSize = n
		return nil
	}
}

// WithDialTimeout sets the dial timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Config) error { c.DialTimeout = d; return nil }
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(d time.Duration) Option {
	return func(c *Config) error { c.ReadTimeout = d; return nil }
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(d time.Duration) Option {
	return func(c *Config) error { c.WriteTimeout = d; return nil }
}

// WithPingTimeout bounds the startup ping. Use 0 to disable the check.
func WithPingTimeout(d time.Duration) Option {
	return func(c *Config) error { c.PingTimeout = d; return nil }
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
