package raw

import (
	"net/http"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

func (p *Processor) Init(conn *redis.Client, config map[string]interface{}) {
	p.conn = conn
	p.tempDir = config["tempDir"].(string)
	p.cacheTTL = config["cacheTTL"].(int64)
}

func (p *Processor) Terminate() {

}

func (p *Processor) GetThumbnail(w http.ResponseWriter, path string) {
	thumb, err := p.getNEFPreview(path)
	if err != nil {
		log.Info("NEF processor error ", err.Error())
		w.WriteHeader(500)
		return
	}

	w.Write(thumb)
}
