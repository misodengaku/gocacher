package ffmpeg

import "github.com/go-redis/redis"

type Processor struct {
	cacheTTL int
	conn     *redis.Client
}
