package ffmpeg

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

var tempDir string

func (p *Processor) Init(conn *redis.Client, config map[string]interface{}) {
	p.conn = conn
	p.cacheTTL = config["cacheTTL"].(int)
	// p.workers = make(Worker, 1)

	// mutex = new(sync.Mutex)
}

func (p *Processor) Terminate() {

}

func (p *Processor) GetThumbnail(w http.ResponseWriter, path string) {
	gif := getMP4Thumbnail(path, 300)
	log.Info(gif)
	gifImg, err := ioutil.ReadFile(gif)
	if err != nil {
		log.Info("gif load error ", err.Error())
		w.WriteHeader(500)
		return
	}

	go func(_path string, img []byte, cacheTTL int) {
		status := p.conn.Set(_path, img, time.Duration(cacheTTL)*time.Second)
		if status.Err() != nil {
			log.Fatal("set fail", status.Err())
		}
	}(path, gifImg, p.cacheTTL)

	w.Header().Set("Content-Type", "image/gif")
	w.Write(gifImg)
}
