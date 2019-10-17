package raw

import "github.com/go-redis/redis"

type Processor struct {
	tempDir  string
	cacheTTL int64
	conn     *redis.Client
}
