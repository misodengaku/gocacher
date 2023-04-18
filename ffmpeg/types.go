package ffmpeg

import "github.com/go-redis/redis/v8"

type Processor struct {
	cacheTTL int
	conn     *redis.Client
	tempDir  string
}
