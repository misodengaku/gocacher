package imagick

import (
	"net/http"
	"sync"

	"github.com/go-redis/redis"
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

	thumb, err := worker.processGenericImage(path) // log.Info(path, ":\tCache set")

	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(thumb)
}

func (p *Processor) GetProcessableFileExts() []string {
	return []string{"*"}
}
