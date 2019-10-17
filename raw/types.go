package raw

import "github.com/go-redis/redis"

type Processor struct {
	tempDir  string
	cacheTTL int
	conn     *redis.Client
}
