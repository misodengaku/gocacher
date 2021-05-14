package main

import (
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/misodengaku/gocacher/ffmpeg"
	"github.com/misodengaku/gocacher/imagick"
	"github.com/misodengaku/gocacher/processor"
	"github.com/misodengaku/gocacher/raw"
)

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gocacher_cache_hit_count",
		Help: "The total number of cache hit",
	})
	processors []processor.Processor
	conn       *redis.Client
	config     Config
)

func handler(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(config.FsRoot, r.URL.Path)
	log.WithFields(log.Fields{
		"path": r.URL.Path,
	}).Info("GET")
	exists, err := conn.Exists(path).Result()
	if err != nil {
		log.Warn(err)
	}

	// disable cache
	// exists = 0

	if exists == 1 {
		start := time.Now()
		log.WithFields(log.Fields{
			"path": r.URL.Path,
		}).Info("Cache hit")
		thumb, _ := conn.Get(path).Bytes()
		if string(thumb[:4]) == "JFIF" {
			w.Header().Set("Content-Type", "image/jpeg")
		} else {
			w.Header().Set("Content-Type", "image/gif")
		}
		w.Write(thumb)
		end := time.Now()
		ms := int64((end.Sub(start)) / time.Microsecond)
		// log.Info(start)
		// log.Info(end)

		log.WithFields(log.Fields{
			"path":     r.URL.Path,
			"duration": ms,
		}).Info("Contents responded (from cache)")
		return
	}

	// start := time.Now()

	fileext := strings.ToUpper(strings.Trim(filepath.Ext(path), "."))
	var fallback processor.Processor
	fallback = nil

	for _, p := range processors {
		pa := p.GetProcessableFileExts()
		for _, v := range pa {
			if v == fileext {
				p.GetThumbnail(w, path)
				return
			}
			if fallback == nil && v == "*" {
				fallback = p
				break
			}
		}
	}

	if fallback != nil {
		fallback.GetThumbnail(w, path)
	}

	log.Info("can't find any suitable processor")
	w.WriteHeader(500)
}

func main() {

	tempDir, err := ioutil.TempDir("", "gocacher")
	if err != nil {
		log.Fatal(err)
	}

	processors = []processor.Processor{
		new(imagick.Processor),
		new(ffmpeg.Processor),
		new(raw.Processor),
	}

	configBin, err := ioutil.ReadFile("/etc/gocacher/config.yml")
	if err != nil {
		panic("config read error: " + err.Error())
	}

	err = yaml.Unmarshal(configBin, &config)
	if err != nil {
		panic("config unmarshal error: " + err.Error())
	}

	conn = redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer conn.Close()

	_, err = conn.Ping().Result()
	if err != nil {
		log.Fatal("connection fail: redis")
	}

	pc := map[string]interface{}{
		"cacheTTL": config.CacheTTL,
		"tempDir":  tempDir,
	}
	for _, v := range processors {
		v.Init(conn, pc)
	}

	mainServer := http.NewServeMux()
	mainServer.HandleFunc("/", handler)

	// promhttp
	promServer := http.NewServeMux()
	promServer.Handle("/metrics", promhttp.Handler())

	log.Infoln("start listen")

	go func() {
		// promhttp
		log.Println(http.ListenAndServe(config.PromHTTPListenAddr, promServer))
	}()

	go func() {
		// pprof
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	http.ListenAndServe(config.ListenAddr, mainServer)
}
