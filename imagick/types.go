package imagick

import (
	"sync"

	"github.com/go-redis/redis"
	"gopkg.in/gographics/imagick.v2/imagick"
)

type Processor struct {
	workers  []Worker
	cacheTTL int
	conn     *redis.Client
	pool     sync.Pool
	limit    chan struct{}
}

type Worker struct {
	mutex *sync.Mutex
	mw    *imagick.MagickWand
	p     *Processor
}

func (w *Worker) Init(p *Processor) {
	w.p = p
	w.mutex = new(sync.Mutex)
	w.mw = imagick.NewMagickWand()
}
