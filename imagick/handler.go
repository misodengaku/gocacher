package imagick

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/gographics/imagick.v2/imagick"
)

// const WorkerCount int = 16

// var s chan string

func (p *Processor) Init(conn *redis.Client, config map[string]interface{}) {
	imagick.Initialize()
	p.cacheTTL = config["cacheTTL"].(int)
	p.conn = conn

	p.pool = sync.Pool{New: func() interface{} {
		w := Worker{}
		w.Init(p)
		return w
	}}

}

func (p *Processor) Terminate() {

	imagick.Terminate()
}

func (p *Processor) GetThumbnail(w http.ResponseWriter, path string) {
	// TODO: select worker
	// worker := p.workers[0]

	worker := p.Get()
	defer p.Put(worker)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	thumb, err := worker.processGenericImage(ctx, path)
	cancel()
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(thumb)
}

func (p *Processor) GetProcessableFileExts() []string {
	return []string{"JPG", "JPEG", "PNG", "GIF"}
}
