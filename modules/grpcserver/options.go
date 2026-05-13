package grpcserver

import (
	"errors"
	"time"

	"google.golang.org/grpc"
)

// Config is the resolved configuration.
type Config struct {
	Addr        string
	Reflection  bool
	Health      bool
	StopTimeout time.Duration
	ServerOpts  []grpc.ServerOption
}

// Option mutates Config.
type Option func(*Config) error

// WithAddr sets the listen address.
func WithAddr(s string) Option {
	return func(c *Config) error {
		if s == "" {
			return errors.New("grpcserver: addr is empty")
		}
		c.Addr = s
		return nil
	}
}

// WithReflection enables the reflection service.
func WithReflection(v bool) Option {
	return func(c *Config) error { c.Reflection = v; return nil }
}

// WithHealth enables the standard grpc.health.v1 service.
func WithHealth(v bool) Option {
	return func(c *Config) error { c.Health = v; return nil }
}

// WithStopTimeout sets the GracefulStop timeout before falling back to Stop.
func WithStopTimeout(d time.Duration) Option {
	return func(c *Config) error { c.StopTimeout = d; return nil }
}

// WithServerOptions appends raw grpc.ServerOption values.
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(c *Config) error {
		c.ServerOpts = append(c.ServerOpts, opts...)
		return nil
	}
}

func defaultConfig() *Config {
	return &Config{
		Addr:        DefaultAddr,
		Reflection:  DefaultReflection,
		Health:      DefaultHealth,
		StopTimeout: 10 * time.Second,
	}
}
