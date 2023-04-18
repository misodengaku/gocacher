package processor

import (
	"net/http"

	"github.com/go-redis/redis/v8"
)

type Processor interface {
	Init(*redis.Client, map[string]interface{})
	Terminate()
	GetThumbnail(http.ResponseWriter, string)
	GetProcessableFileExts() []string
	// GetMetrics() Metrics
}

type Metrics struct {
	ProcessedCount     uint64
	CacheHitCount      uint64
	UnprocessableCount uint64
}
