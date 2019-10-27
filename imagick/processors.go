package imagick

import (
	"io/ioutil"
	"time"

	log "github.com/sirupsen/logrus"
)

func (p *Processor) Get() Worker {
//	p.limit <- struct{}{}
	return p.pool.Get().(Worker)
}

func (p *Processor) Put(w Worker) {
	p.pool.Put(w)
//	<-p.limit
}

func (w *Worker) processGenericImage(path string) ([]byte, error) {
	// start := time.Now()
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		// log.Warn(err)
		// if strings.Contains(err.Error(), "no such file or directory") {
		// 	w.WriteHeader(404)
		// } else {
		// 	w.WriteHeader(500)
		// }
		return nil, err
	}
	w.mutex.Lock()
	err = w.mw.ReadImageBlob(bs)
	if err != nil {
		// w.WriteHeader(500)
		return nil, err
	}
	thumb := w.getThumbnailFromBlob(300, 300)
	w.mutex.Unlock()

	// end_conv := time.Now()

	status := w.p.conn.Set(path, thumb, time.Duration(w.p.cacheTTL)*time.Second)
	if status.Err() != nil {
		log.Fatal("set fail")
	}

	// end_set := time.Now()
	// conv_ms := int64((end_conv.Sub(start)) / time.Microsecond)
	// set_ms := int64((end_set.Sub(end_conv)) / time.Microsecond)

	return thumb, nil
}

func (w *Worker) getThumbnailFromBlob(targetWidth, targetHeight uint) []byte {
	var err error
	width, height := w.mw.GetImageWidth(), w.mw.GetImageHeight()
	resizedWidth, resizedHeight := getResizedWH(width, height, targetWidth, targetHeight)
	err = w.mw.ThumbnailImage(resizedWidth, resizedHeight)
	if err != nil {
		panic(err)
	}

	// 切り抜き
	startX, startY := getStartPointXY(width, height, resizedWidth, resizedHeight)
	err = w.mw.ExtentImage(targetWidth, targetHeight, startX, startY)
	if err != nil {
		panic(err)
	}
	err = w.mw.SetImageCompressionQuality(95)
	if err != nil {
		panic(err)
	}

	return w.mw.GetImageBlob()
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
