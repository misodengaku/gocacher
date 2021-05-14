package ffmpeg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
)

func (p *Processor) getMP4Thumbnail(filename string, targetWidth uint) string {
	hash := sha256.Sum256([]byte(filename))
	fileid := filepath.Join(p.tempDir, hex.EncodeToString(hash[:])+".gif")

	skipSecond := 0
	duration := 10
	fps := 4
	size := 300

	// https://robservatory.com/easily-create-animated-gifs-from-video-via-ffmpeg/
	filters := fmt.Sprintf("fps=%d,scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,pad=%d:%d:(ow-iw)/2:(oh-ih)/2", fps, size, size, size, size)

	cmd1 := []string{
		"-ss", strconv.Itoa(skipSecond),
		"-t", strconv.Itoa(duration),
		"-i", filename,
		"-vf", filters,
		"-y",
		fileid,
	}

	var out1 bytes.Buffer

	log.Info("stage 1")
	movToPaletteCmd := exec.Command("ffmpeg", cmd1...)
	movToPaletteCmd.Stdout = &out1
	err := movToPaletteCmd.Run()
	if err != nil {
		log.Warn(err)
		return ""
	}
	log.Info("encoded: ", fileid)

	return fileid
}
