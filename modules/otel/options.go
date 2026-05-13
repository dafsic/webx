package otel

import (
	"errors"
	"time"
)

// Config is the resolved configuration of the otel module.
type Config struct {
	Endpoint    string
	ServiceName string
	SampleRatio float64
	Insecure    bool
	Disabled    bool
	DialTimeout time.Duration
}

// Option mutates Config and may return an error.
type Option func(*Config) error

// WithEndpoint sets the OTLP gRPC endpoint (host:port).
func WithEndpoint(s string) Option {
	return func(c *Config) error {
		c.Endpoint = s
		return nil
	}
}

// WithServiceName sets the resource service.name attribute.
func WithServiceName(s string) Option {
	return func(c *Config) error {
		if s == "" {
			return errors.New("otel: service name is required")
		}
		c.ServiceName = s
		return nil
	}
}

// WithSampleRatio configures TraceIDRatioBased sampling. 1.0 means always.
func WithSampleRatio(r float64) Option {
	return func(c *Config) error {
		if r < 0 || r > 1 {
			return errors.New("otel: sample ratio must be in [0,1]")
		}
		c.SampleRatio = r
		return nil
	}
}

// WithInsecure toggles plaintext OTLP gRPC.
func WithInsecure(v bool) Option {
	return func(c *Config) error {
		c.Insecure = v
		return nil
	}
}

// WithDisabled fully disables the SDK (use a no-op tracer provider).
func WithDisabled(v bool) Option {
	return func(c *Config) error {
		c.Disabled = v
		return nil
	}
}

// WithDialTimeout caps the OTLP exporter startup.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Config) error {
		c.DialTimeout = d
		return nil
	}
}

func defaultConfig() *Config {
	return &Config{
		Endpoint:    DefaultEndpoint,
		SampleRatio: DefaultSampleRatio,
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	}
}
