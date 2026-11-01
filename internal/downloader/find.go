package downloader

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errNoOutput = errors.New("yt-dlp finished without an output file")

var mediaExt = map[string]struct{}{
	".mp4": {}, ".mkv": {}, ".webm": {}, ".mov": {},
	".m4a": {}, ".mp3": {}, ".opus": {}, ".aac": {}, ".flac": {}, ".ogg": {},
}

// FindOutput picks the finished media file in dir. Partial files are ignored
// so a resume can continue from them.
func FindOutput(dir string) (string, error) {
	var best string
	var bestSize int64
	var bestTime time.Time
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".part") || strings.HasSuffix(lower, ".ytdl") || strings.HasSuffix(lower, ".temp") || strings.HasSuffix(lower, ".json") {
			return nil
		}
		if _, ok := mediaExt[strings.ToLower(filepath.Ext(name))]; !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() <= 0 {
			return nil
		}
		if info.Size() > bestSize || (info.Size() == bestSize && info.ModTime().After(bestTime)) {
			best = path
			bestSize = info.Size()
			bestTime = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if best == "" {
		return "", errNoOutput
	}
	return best, nil
}

// UsableFile reports whether path is a non-empty file we can publish.
func UsableFile(path string) bool {
	if path == "" || strings.HasSuffix(strings.ToLower(path), ".part") {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular() && st.Size() > 0
}
