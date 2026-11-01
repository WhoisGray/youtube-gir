package downloader

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// LineEvent is one useful yt-dlp output line.
type LineEvent struct {
	Kind       string
	Percent    float64
	TotalBytes int64
	SpeedBPS   int64
	HasSpeed   bool
	ETA        string
	Path       string
	Text       string
}

var (
	percentRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)
	ofRe      = regexp.MustCompile(`of\s+~?\s*([\d.]+)\s*(KiB|MiB|GiB|TiB|KB|MB|GB|TB|B)`)
	atRe      = regexp.MustCompile(`at\s+([\d.]+)\s*(KiB|MiB|GiB|TiB|KB|MB|GB|TB|B)/s`)
	etaRe     = regexp.MustCompile(`ETA\s+(\S+)`)
	destRe    = regexp.MustCompile(`\[download\] Destination:\s+(.+)$`)
	mergeRe   = regexp.MustCompile(`\[Merger\] Merging formats into "?(.+?)"?\s*$`)
	extractRe = regexp.MustCompile(`\[ExtractAudio\] Destination:\s+(.+)$`)
	alreadyRe = regexp.MustCompile(`\[download\] (.+) has already been downloaded\s*$`)
)

// ParseLine classifies a yt-dlp stdout or stderr line.
// Progress is printed on stderr when stdout is not a terminal.
func ParseLine(line string) (LineEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return LineEvent{}, false
	}
	if strings.HasPrefix(line, "ERROR:") {
		return LineEvent{Kind: "error", Text: strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))}, true
	}
	if m := mergeRe.FindStringSubmatch(line); m != nil {
		return LineEvent{Kind: "destination", Path: strings.Trim(m[1], `"`)}, true
	}
	if m := extractRe.FindStringSubmatch(line); m != nil {
		return LineEvent{Kind: "destination", Path: strings.TrimSpace(strings.Trim(m[1], `"`))}, true
	}
	if m := alreadyRe.FindStringSubmatch(line); m != nil {
		return LineEvent{Kind: "destination", Path: strings.TrimSpace(m[1])}, true
	}
	if m := destRe.FindStringSubmatch(line); m != nil {
		return LineEvent{Kind: "destination", Path: strings.TrimSpace(strings.Trim(m[1], `"`))}, true
	}
	if m := percentRe.FindStringSubmatch(line); m != nil {
		ev := LineEvent{Kind: "progress", Text: line}
		ev.Percent, _ = strconv.ParseFloat(m[1], 64)
		if om := ofRe.FindStringSubmatch(line); om != nil {
			ev.TotalBytes = scaleBytes(om[1], om[2])
		}
		if am := atRe.FindStringSubmatch(line); am != nil {
			ev.SpeedBPS = scaleBytes(am[1], am[2])
			ev.HasSpeed = true
		}
		if em := etaRe.FindStringSubmatch(line); em != nil && em[1] != "Unknown" && em[1] != "NA" {
			ev.ETA = em[1]
		}
		return ev, true
	}
	return LineEvent{}, false
}

func scaleBytes(num, unit string) int64 {
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	var mul float64
	switch unit {
	case "B":
		mul = 1
	case "KiB", "KB":
		mul = 1024
	case "MiB", "MB":
		mul = 1024 * 1024
	case "GiB", "GB":
		mul = 1024 * 1024 * 1024
	case "TiB", "TB":
		mul = 1024 * 1024 * 1024 * 1024
	default:
		return 0
	}
	return int64(math.Round(f * mul))
}
