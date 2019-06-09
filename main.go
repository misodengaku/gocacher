package main

import (
	"bytes"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"golang.org/x/image/tiff"
	"gopkg.in/gographics/imagick.v2/imagick"
	"gopkg.in/yaml.v2"
)

type Config struct {
	FsRoot     string `yaml:"fs_root"`
	CacheTTL   int    `yaml:"cache_ttl"`
	maxWorkers int    `yaml:"max_workers"`
	maxQueues  int    `yaml:"max_queues"`
	ListenAddr string `yaml:"listen_addr"`
	RedisAddr  string `yaml:"redis_addr"`
}

var conn *redis.Client
var mw *imagick.MagickWand
var mutex *sync.Mutex
var config Config

var RawImageExts = []string{"NEF", "CR2", "ARW"}

func handler(w http.ResponseWriter, r *http.Request) {

	path := filepath.Join(config.FsRoot, r.URL.Path)
	log.Info(path, ":\t", r.Method)
	exists, err := conn.Exists(path).Result()
	if err != nil {
		log.Warn(err)
	}

	log.Info(path, ":", exists)

	// disable cache
	// exists = 0

	if exists == 1 {
		start := time.Now()
		log.Info(path, ":\tCache hit")
		thumb, _ := conn.Get(path).Bytes()
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(thumb)
		end := time.Now()
		ms := int64((end.Sub(start)) / time.Microsecond)
		// log.Info(start)
		// log.Info(end)
		log.Info(path, ":\tContents responded (from cache) ", ms, "ms")
		return
	}

	// start := time.Now()

	filetype := strings.ToUpper(strings.Trim(filepath.Ext(path), "."))
	for _, v := range RawImageExts {
		if v == filetype {
			ReturnRAWPreview(w, path)
			return
		}
	}

	ReturnGenericImage(w, path)
	// fmt.Fprintf(w, "Hello, World")
}

func ReturnRAWPreview(w http.ResponseWriter, path string) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	tiffImg, err := tiff.Decode(file)
	if err != nil {
		log.Warn("TIFF decode error:", err)
	}

	jpegImg := new(bytes.Buffer)
	err = jpeg.Encode(jpegImg, tiffImg, nil)
	if err != nil {
		log.Warn("JPEG encode error:", err)
	}

	go func(p string, imgBuf *bytes.Buffer) {
		status := conn.Set(path, imgBuf.Bytes(), time.Duration(config.CacheTTL)*time.Second)
		if status.Err() != nil {
			log.Fatal("set fail", status.Err())
		}

	}(path, jpegImg)

	w.Write(jpegImg.Bytes())
	// jpeg.Encode(w, img, nil)
}

func ReturnGenericImage(w http.ResponseWriter, path string) {
	start := time.Now()
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

	end_conv := time.Now()

	status := conn.Set(path, thumb, time.Duration(config.CacheTTL)*time.Second)
	if status.Err() != nil {
		log.Fatal("set fail")
	}

	end_set := time.Now()
	conv_ms := int64((end_conv.Sub(start)) / time.Microsecond)
	set_ms := int64((end_set.Sub(end_conv)) / time.Microsecond)
	// log.Info(path, ":\tCache set")
	log.Info(path, ":\tCache set convert:", conv_ms, "ms, set:", set_ms)

	log.Info(path, ":\tContents responded")
	w.Write(thumb)
}

func main() {

	configBin, err := ioutil.ReadFile("/etc/gocacher/config.yml")
	if err != nil {
		panic("config read error: " + err.Error())
	}
	err = yaml.Unmarshal(configBin, &config)
	if err != nil {
		panic("config unmarshal error: " + err.Error())
	}

	mutex = new(sync.Mutex)

	imagick.Initialize()
	defer imagick.Terminate()

	mw = imagick.NewMagickWand()
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

	http.HandleFunc("/", handler) // ハンドラを登録してウェブページを表示させる

	log.Infoln("start listen")
	http.ListenAndServe(config.ListenAddr, nil)
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
