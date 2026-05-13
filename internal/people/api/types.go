package api

import "github.com/redis/go-redis/v9"

// redisClient is a local alias so module.go signatures stay short.
type redisClient = redis.UniversalClient
