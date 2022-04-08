package pdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
)

func (p *Processor) getPDFThumbnail(filename string, targetWidth uint) (string, string) {
	hash := sha256.Sum256([]byte(filename))
	hashStr := hex.EncodeToString(hash[:])
	// fileid := filepath.Join(p.tempDir, hex.EncodeToString(hash[:])+".gif")

	cmd1 := []string{
		"-singlefile",
		"-jpeg",
		"-scale-to", strconv.FormatUint(uint64(targetWidth), 10),
		filename,
		hashStr,
	}

	var out1 bytes.Buffer

	genThumbCmd := exec.Command("pdftoppm", cmd1...)
	genThumbCmd.Stdout = &out1
	err := genThumbCmd.Run()
	if err != nil {
		log.Warn(err)
		return "", ""
	}
	outname := fmt.Sprintf("%s.jpg", hashStr)
	log.Info("encoded: ", outname)

	return outname, "image/jpeg"
}
