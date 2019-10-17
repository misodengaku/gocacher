package imagick

import (
	"net/http"

	"github.com/go-redis/redis"
	"gopkg.in/gographics/imagick.v2/imagick"
)

func (p *Processor) Init(conn *redis.Client, config map[string]interface{}) {
	imagick.Initialize()
	p.cacheTTL = config["cacheTTL"].(int)
	p.conn = conn

	p.workers = make([]Worker, 1)
	p.workers[0].Init(p)
}

func (p *Processor) Terminate() {

	imagick.Terminate()
}

func (p *Processor) GetThumbnail(w http.ResponseWriter, path string) {
	// TODO: select worker
	worker := p.workers[0]

	thumb, err := worker.processGenericImage(path) // log.Info(path, ":\tCache set")

	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(thumb)
}
