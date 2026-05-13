package ginserver

import (
	"time"
)

// Config is the resolved configuration.
type Config struct {
	Addr           string
	Mode           string
	StopTimeout    time.Duration
	TrustedProxies []string
}

// Option mutates Config.
type Option func(*Config) error

// WithAddr sets the listen address.
func WithAddr(s string) Option {
	return func(c *Config) error { c.Addr = s; return nil }
}

// WithMode sets gin's mode (debug/release/test).
func WithMode(s string) Option {
	return func(c *Config) error { c.Mode = s; return nil }
}

// WithStopTimeout caps server.Shutdown.
func WithStopTimeout(d time.Duration) Option {
	return func(c *Config) error { c.StopTimeout = d; return nil }
}

// WithTrustedProxies configures gin's SetTrustedProxies; nil disables.
func WithTrustedProxies(p []string) Option {
	return func(c *Config) error { c.TrustedProxies = p; return nil }
}

func defaultConfig() *Config {
	return &Config{
		Addr:        DefaultAddr,
		Mode:        DefaultMode,
		StopTimeout: 10 * time.Second,
	}
}
