package raw

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/image/tiff"
)

func (p *Processor) getNEFPreview(path string) ([]byte, error) {
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

	go func(_path string, imgBuf *bytes.Buffer, cacheTTL int) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		status := p.conn.Set(ctx, _path, imgBuf.Bytes(), time.Duration(cacheTTL)*time.Second)
		cancel()
		if status.Err() != nil {
			log.Fatal("set fail", status.Err())
		}
	}(path, jpegImg, p.cacheTTL)

	return jpegImg.Bytes(), nil
}
