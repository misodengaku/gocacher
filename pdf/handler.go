package pdf

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

func (p *Processor) Init(conn *redis.Client, config map[string]interface{}) {
	p.conn = conn
	p.cacheTTL = config["cacheTTL"].(int)
	p.tempDir = config["tempDir"].(string)
	// p.workers = make(Worker, 1)

	// mutex = new(sync.Mutex)
}

func (p *Processor) Terminate() {

}

func (p *Processor) GetThumbnail(w http.ResponseWriter, path string) {
	thumb, mime := p.getPDFThumbnail(path, 300)
	log.Info(thumb, mime)
	thumbImg, err := ioutil.ReadFile(thumb)
	if err != nil {
		log.Info("thumbnail load error ", err.Error())
		w.WriteHeader(500)
		return
	}

	go func(_path string, img []byte, cacheTTL int) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		status := p.conn.Set(ctx, _path, img, time.Duration(cacheTTL)*time.Second)
		cancel()
		if status.Err() != nil {
			log.Fatal("set fail: ", status.Err())
		}
	}(path, thumbImg, p.cacheTTL)

	w.Header().Set("Content-Type", mime)
	w.Write(thumbImg)
}

func (p *Processor) GetProcessableFileExts() []string {
	return []string{"PDF"}
}
