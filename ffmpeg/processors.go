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
	palette := filepath.Join(p.tempDir, hex.EncodeToString(hash[:])+".png")

	skipSecond := 0
	duration := 10
	fps := 4
	size := 300

	// https://robservatory.com/easily-create-animated-gifs-from-video-via-ffmpeg/
	filters := fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos,palettegen", fps, size)

	cmd1 := []string{
		"-ss", strconv.Itoa(skipSecond),
		"-t", strconv.Itoa(duration),
		"-i", filename,
		"-vf", filters,
		"-y",
		palette}

	filters = fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos [x]; [x][1:v] paletteuse", fps, size)

	cmd2 := []string{
		"-ss", strconv.Itoa(skipSecond),
		"-t", strconv.Itoa(duration),
		"-i", filename,
		"-i", palette,
		"-lavfi",
		filters,
		"-y",
		fileid}

	for _, v := range cmd2 {
		fmt.Print(v)
		fmt.Print(" ")
	}
	fmt.Println("")

	var out1 bytes.Buffer

	log.Info("stage 1")
	movToPaletteCmd := exec.Command("ffmpeg", cmd1...)
	movToPaletteCmd.Stdout = &out1
	err := movToPaletteCmd.Run()
	if err != nil {
		log.Warn(err)
		return ""
	}

	// cmd1Out, cmd1Err := exec.Command("/usr/bin/ffmpeg", cmd1...).Output()
	// log.Info(cmd1Out, cmd1Err)
	// cmd2Out, cmd2Err := exec.Command("/usr/bin/ffmpeg", cmd2...).Output()

	log.Info("stage 2")
	paletteToGifCmd := exec.Command("ffmpeg", cmd2...)
	// paletteToGifCmd.Stdout = &out2
	// err = paletteToGifCmd.Run()
	cmd2Out, cmd2Err := paletteToGifCmd.Output()
	log.Info(cmd2Out, cmd2Err)
	if cmd2Err != nil {
		log.Warn("stage2 failed: ", filename)
		log.Warn(cmd2Err)
		return ""
	}


	return fileid
}
