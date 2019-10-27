package processor

import (
	"net/http"

	"github.com/go-redis/redis"
)

type Processor interface {
	Init(*redis.Client, map[string]interface{})
	Terminate()
	GetThumbnail(http.ResponseWriter, string)
}
