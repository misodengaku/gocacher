package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	"github.com/misodengaku/gocacher/pdf"
	"github.com/misodengaku/gocacher/processor"
	"github.com/misodengaku/gocacher/raw"
)

var (
	cacheHitCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gocacher_cache_hit_count",
		Help: "The total number of cache hit",
	})
	processedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gocacher_processed_count",
		Help: "The total number of processed requests",
	})
	failedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gocacher_failed_count",
		Help: "The total number of failed requests",
	})
	processors []processor.Processor
	conn       *redis.Client
	config     Config
)

func handler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	path := filepath.Join(config.FsRoot, urlPath)
	log.WithFields(log.Fields{
		"path":     r.URL.Path,
		"realpath": path,
	}).Info("GET")

	fi, err := os.Stat(path)
	if err != nil {
		log.Warn(err)
		w.WriteHeader(404)
		return
	}
	if fi.IsDir() {
		log.WithFields(log.Fields{
			"path": path,
		}).Info("GET dirlist")
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			log.Warn(err)
			w.WriteHeader(500)
			return
		}
		filelist := make([]NginxCompatibleFileInfo, 0, len(dirEntries))
		dirlist := make([]NginxCompatibleFileInfo, 0, len(dirEntries))
		wg := &sync.WaitGroup{}
		wg.Add(len(dirEntries))
		for i, v := range dirEntries {
			go func(index int, file fs.DirEntry) {
				fsinfo, _ := file.Info()
				if fsinfo.IsDir() {
					dirlist = append(dirlist, NginxCompatibleFileInfo{
						Name:            file.Name(),
						Type:            "directory",
						ModifiedTime:    fsinfo.ModTime().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"),
						ModifiedTimeRaw: fsinfo.ModTime().UTC(),
					})
				} else {
					filelist = append(filelist, NginxCompatibleFileInfo{
						Name:            file.Name(),
						Type:            "file",
						ModifiedTime:    fsinfo.ModTime().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"),
						ModifiedTimeRaw: fsinfo.ModTime().UTC(),
						Size:            fsinfo.Size(),
					})
				}
				wg.Done()
			}(i, v)
		}
		wg.Wait()
		wg.Add(2)
		go func() {
			sort.Slice(dirlist, func(i, j int) bool { return strings.ToLower(dirlist[i].Name) < strings.ToLower(dirlist[j].Name) })
			wg.Done()
		}()
		go func() {
			switch r.FormValue("order") {
			case "size":
				sort.Slice(filelist, func(i, j int) bool { return filelist[i].Size < filelist[j].Size })
			case "size-reverse":
				sort.Slice(filelist, func(i, j int) bool { return filelist[i].Size > filelist[j].Size })
			case "date":
				sort.Slice(filelist, func(i, j int) bool {
					return filelist[i].ModifiedTimeRaw.UnixNano() < filelist[j].ModifiedTimeRaw.UnixNano()
				})
			case "date-reverse":
				sort.Slice(filelist, func(i, j int) bool {
					return filelist[i].ModifiedTimeRaw.UnixNano() > filelist[j].ModifiedTimeRaw.UnixNano()
				})
			case "name-reverse":
				sort.Slice(filelist, func(i, j int) bool { return strings.ToLower(filelist[i].Name) > strings.ToLower(filelist[j].Name) })
			default:
				sort.Slice(filelist, func(i, j int) bool { return strings.ToLower(filelist[i].Name) < strings.ToLower(filelist[j].Name) })
			}

			wg.Done()
		}()
		wg.Wait()
		respList := append(dirlist, filelist...)
		resp, err := json.Marshal(respList)
		if err != nil {
			log.Warn(err)
			w.WriteHeader(500)
			return
		}
		w.Write(resp)
		return
	}

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
		cacheHitCounter.Inc()
		processedCounter.Inc()
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
				processedCounter.Inc()
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
		processedCounter.Inc()
		return
	}

	log.Info("can't find any suitable processor")
	w.WriteHeader(500)
}

func main() {

	tempDir, err := os.MkdirTemp(os.TempDir(), "gocacher")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}

	processors = []processor.Processor{
		new(imagick.Processor),
		new(ffmpeg.Processor),
		new(raw.Processor),
		new(pdf.Processor),
	}

	configBin, err := os.ReadFile("/etc/gocacher/config.yml")
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
