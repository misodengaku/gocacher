package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/go-redis/redis/v8"
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
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			log.Warn(err)
			w.WriteHeader(500)
			return
		}
		filelist := make([]NginxCompatibleFileInfo, 0, len(dirEntries))
		filelistMutex := sync.Mutex{}
		dirlist := make([]NginxCompatibleFileInfo, 0, len(dirEntries))
		dirlistMutex := sync.Mutex{}
		wg := &sync.WaitGroup{}
		wg.Add(len(dirEntries))
		for i, v := range dirEntries {
			go func(index int, file fs.DirEntry) {
				fsinfo, _ := file.Info()
				if fsinfo.IsDir() {
					dirlistMutex.Lock()
					dirlist = append(dirlist, NginxCompatibleFileInfo{
						Name:            file.Name(),
						Type:            "directory",
						ModifiedTime:    fsinfo.ModTime().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"),
						ModifiedTimeRaw: fsinfo.ModTime().UTC(),
					})
					dirlistMutex.Unlock()
				} else {
					filelistMutex.Lock()
					filelist = append(filelist, NginxCompatibleFileInfo{
						Name:            file.Name(),
						Type:            "file",
						ModifiedTime:    fsinfo.ModTime().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"),
						ModifiedTimeRaw: fsinfo.ModTime().UTC(),
						Size:            fsinfo.Size(),
					})
					filelistMutex.Unlock()
				}
				wg.Done()
			}(i, v)
		}
		wg.Wait()

		// sort dirlist
		sort.Slice(dirlist, func(i, j int) bool { return strings.ToLower(dirlist[i].Name) < strings.ToLower(dirlist[j].Name) })

		// sort filelist
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

		respList := append(dirlist, filelist...)
		startAt, err := strconv.Atoi(r.FormValue("start_at"))
		if err != nil {
			startAt = 0
		}
		size, err := strconv.Atoi(r.FormValue("size"))
		if err != nil {
			size = len(filelist)
		}
		if startAt >= len(respList) {
			startAt = 0
		}
		if startAt+size >= len(respList) {
			size = len(respList) - startAt
		}
		resp, err := json.Marshal(respList[startAt : startAt+size])
		log.WithFields(log.Fields{
			"path":        path,
			"start_at":    startAt,
			"size":        size,
			"dir_entries": len(dirEntries),
		}).Info("GET dirlist")
		if err != nil {
			log.Warn(err)
			w.WriteHeader(500)
			return
		}
		w.Write(resp)
		return
	}

	exists, err := conn.Exists(r.Context(), path).Result()
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
		thumb, _ := conn.Get(r.Context(), path).Bytes()
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

	_, err = conn.Ping(context.Background()).Result()
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

	log.Errorln(http.ListenAndServe(config.ListenAddr, mainServer))
}
