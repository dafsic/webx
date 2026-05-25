package redis

import "time"

// Default values and identifiers for the redis module.
const (
	ModuleName = "redis"

	DefaultAddr         = "127.0.0.1:6379"
	DefaultDB           = 0
	DefaultPoolSize     = 10
	DefaultDialTimeout  = 5 * time.Second
	DefaultReadTimeout  = 3 * time.Second
	DefaultWriteTimeout = 3 * time.Second
	DefaultPingTimeout  = 5 * time.Second
)
