package ffmpeg

import "github.com/go-redis/redis"

type Processor struct {
	cacheTTL int64
	conn     *redis.Client
}
