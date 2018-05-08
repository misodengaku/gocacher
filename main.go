package main

import (
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gographics/imagick.v3/imagick"
)

const (
	FsRoot     = "/storage1"
	CacheTTL   = 86400 * time.Second
	maxWorkers = 30
	maxQueues  = 10000
)

var conn *redis.Client
var mw *imagick.MagickWand
var mutex *sync.Mutex

func handler(w http.ResponseWriter, r *http.Request) {

	path := filepath.Join(FsRoot, r.URL.Path)
	log.Info(path, ":\t", r.Method)
	exists, _ := conn.Exists(path).Result()

	// disable cache
	// exists = 0

	if exists == 1 {
		start := time.Now()
		log.Info(path, ":\tCache hit")
		thumb, _ := conn.Get(path).Bytes()
		w.Write(thumb)
		end := time.Now()
		ms := int64((end.Sub(start)) / time.Microsecond)
		log.Info(start)
		log.Info(end)
		log.Info(path, ":\tContents responded (from cache) ", ms, "ms")
		return
	}

	bs, err := ioutil.ReadFile(path)
	if err != nil {
		log.Warn(err)
		if strings.Contains(err.Error(), "no such file or directory") {
			w.WriteHeader(404)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	mutex.Lock()
	err = mw.ReadImageBlob(bs)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	thumb := GetThumbnailFromBlob(300, 300)
	mutex.Unlock()

	go func(key string, data []byte) {
		status := conn.Set(key, data, CacheTTL)
		// conn.Do("SETEX", key, CacheTTL, data)
		if status.Err() != nil {
			log.Fatal("set fail")
		}
		conn.FlushAll()
		log.Info(key, ":\tCache set")
	}(path, thumb)

	log.Info(path, ":\tContents responded")
	w.Write(thumb)
	// fmt.Fprintf(w, "Hello, World")
}

func main() {
	var err error
	mutex = new(sync.Mutex)

	imagick.Initialize()
	defer imagick.Terminate()

	mw = imagick.NewMagickWand()
	conn = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer conn.Close()

	_, err = conn.Ping().Result()
	if err != nil {
		log.Fatal("connection fail: redis")
	}

	http.HandleFunc("/", handler) // ハンドラを登録してウェブページを表示させる

	log.Infoln("start listen")
	http.ListenAndServe(":8081", nil)
}

/*
type (
	// Dispatcher represents a management workers.
	Dispatcher struct {
		pool    chan *worker     // Idle 状態の worker の受け入れ先
		queue   chan interface{} // メッセージの受け入れ先
		workers []*worker
		wg      sync.WaitGroup // 非同期処理の待機用
		quit    chan struct{}
	}
)

type (
	// worker represents the worker that executes the job.
	worker struct {
		dispatcher *Dispatcher
		data       chan interface{} // 受け取ったメッセージの受信先
		quit       chan struct{}
		mw         *imagick.MagickWand
	}
)
func (w *worker) start() {
	go func() {
		for {
			// dispatcher の pool に自身を送信する（Idle状態を示す）
			w.dispatcher.pool <- w

			select {
			// メッセージがキューイングされた場合、 v にメッセージを設定
			case v := <-w.data:
				if str, ok := v.(string); ok {
					// get 関数でHTTPリクエスト
					w.mw
				}

				// WaitGroupのカウントダウン
				w.dispatcher.wg.Done()

			case <-w.quit:
				return
			}
		}
	}()
}

// NewDispatcher returns a pointer of Dispatcher.
func NewImageDispatcher() *Dispatcher {
	// dispatcher の初期化
	d := &Dispatcher{
		pool:  make(chan *worker, maxWorkers),    // capacity は用意する worker の数
		queue: make(chan interface{}, maxQueues), // capacity はメッセージをキューイングする数
		quit:  make(chan struct{}),
	}

	// worker の初期化
	d.workers = make([]*worker, cap(d.pool))

	for i := 0; i < cap(d.pool); i++ {
		w := worker{
			dispatcher: d,
			data:       make(chan interface{}), // worker でキューイングする場合は capacity を2以上
			quit:       make(chan struct{}),
			mw: imagick.NewMagickWand()
		}
		d.workers[i] = &w
	}
	return d
}
*/

func GetThumbnailFromBlob(targetWidth, targetHeight uint) []byte {
	var err error
	width, height := mw.GetImageWidth(), mw.GetImageHeight()
	resizedWidth, resizedHeight := getResizedWH(width, height, targetWidth, targetHeight)
	err = mw.ThumbnailImage(resizedWidth, resizedHeight)
	if err != nil {
		panic(err)
	}

	// 切り抜き
	startX, startY := getStartPointXY(width, height, resizedWidth, resizedHeight)
	err = mw.ExtentImage(targetWidth, targetHeight, startX, startY)
	if err != nil {
		panic(err)
	}
	err = mw.SetImageCompressionQuality(95)
	if err != nil {
		panic(err)
	}

	return mw.GetImageBlob()
}

func getResizedWH(width, height, targetWidth, targetHeight uint) (resizedWidth, resizedHeight uint) {
	if width < height {
		ratio := float32(width) / float32(height)
		targetHeight = uint(float32(targetWidth) / ratio)
	} else {
		ratio := float32(height) / float32(width)
		targetWidth = uint(float32(targetHeight) / ratio)
	}
	return targetWidth, targetHeight
}

func getStartPointXY(width, height, resizedWidth, resizedHeight uint) (x, y int) {
	startX, startY := 0, 0
	if width < height {
		startY = int((float32(resizedHeight) - float32(resizedWidth)) / 2.0)
	} else {
		startX = int((float32(resizedWidth) - float32(resizedHeight)) / 2.0)
	}
	return startX, startY
}
