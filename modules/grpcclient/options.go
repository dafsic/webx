package grpcclient

import (
	"time"

	"google.golang.org/grpc"
)

// Config carries dialer-wide defaults applied to every Dial.
type Config struct {
	DialTimeout     time.Duration
	KeepaliveTime   time.Duration
	KeepaliveJitter time.Duration
	ExtraDialOpts   []grpc.DialOption
}

// Option mutates Config.
type Option func(*Config) error

// WithDialTimeout sets the per-dial timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Config) error { c.DialTimeout = d; return nil }
}

// WithKeepalive sets keepalive ping interval.
func WithKeepalive(d time.Duration) Option {
	return func(c *Config) error { c.KeepaliveTime = d; return nil }
}

// WithDialOptions appends raw grpc.DialOption values.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(c *Config) error {
		c.ExtraDialOpts = append(c.ExtraDialOpts, opts...)
		return nil
	}
}

func defaultConfig() *Config {
	return &Config{
		DialTimeout:   5 * time.Second,
		KeepaliveTime: 30 * time.Second,
	}
}
