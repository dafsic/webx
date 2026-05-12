package redis

import "time"

// Default values and identifiers for the redis module.
const (
	ModuleName = "redis"

	FlagAddrs      = "redis-addrs"
	FlagDB         = "redis-db"
	FlagUsername   = "redis-username"
	FlagPassword   = "redis-password"
	FlagMasterName = "redis-master-name"
	FlagPoolSize   = "redis-pool-size"
	FlagDialTimeout  = "redis-dial-timeout"
	FlagReadTimeout  = "redis-read-timeout"
	FlagWriteTimeout = "redis-write-timeout"

	EnvAddrs        = "REDIS_ADDRS"
	EnvDB           = "REDIS_DB"
	EnvUsername     = "REDIS_USERNAME"
	EnvPassword     = "REDIS_PASSWORD"
	EnvMasterName   = "REDIS_MASTER_NAME"
	EnvPoolSize     = "REDIS_POOL_SIZE"
	EnvDialTimeout  = "REDIS_DIAL_TIMEOUT"
	EnvReadTimeout  = "REDIS_READ_TIMEOUT"
	EnvWriteTimeout = "REDIS_WRITE_TIMEOUT"

	DefaultAddr         = "127.0.0.1:6379"
	DefaultDB           = 0
	DefaultPoolSize     = 10
	DefaultDialTimeout  = 5 * time.Second
	DefaultReadTimeout  = 3 * time.Second
	DefaultWriteTimeout = 3 * time.Second
	DefaultPingTimeout  = 5 * time.Second

	// OptionsGroup is the fx group tag callers can use to inject extra
	// Options into the redis configuration.
	OptionsGroup = "redis_options"
)
