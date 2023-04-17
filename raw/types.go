package raw

import "github.com/go-redis/redis/v8"

type Processor struct {
	tempDir  string
	cacheTTL int
	conn     *redis.Client
}
