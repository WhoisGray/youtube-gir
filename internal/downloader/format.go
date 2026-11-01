package downloader

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Standard heights are sent by the extension quality menu.
// Other all-digit values are treated as yt-dlp format ids (for example 18, 22, 137).
var standardHeights = map[string]struct{}{
	"144": {}, "240": {}, "360": {}, "480": {},
	"720": {}, "1080": {}, "1440": {}, "2160": {}, "4320": {},
}

var formatIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_+.-]{0,80}$`)

// Car players take H.264 up to 1080p and AAC in an MP4.
const (
	QualityCar = "car"
	carFormat  = "bestvideo[vcodec^=avc1][height<=1080]+bestaudio[acodec=aac]/bestvideo[vcodec^=avc1][height<=1080]+bestaudio/bestvideo[height<=1080]+bestaudio/best"
	carMerge   = "Merger+ffmpeg:-c:v copy -c:a aac -b:a 192k -ar 44100 -movflags +faststart"
)

// NormalizeQuality returns "best", "car", a standard height, or a format id.
func NormalizeQuality(kind, quality string) (string, error) {
	if kind != "video" && kind != "audio" {
		return "", errors.New("kind must be video or audio")
	}
	if kind == "audio" {
		return "best", nil
	}
	q := strings.TrimSpace(quality)
	if q == "" || strings.EqualFold(q, "best") {
		return "best", nil
	}
	if strings.EqualFold(q, QualityCar) {
		return QualityCar, nil
	}
	if _, ok := standardHeights[q]; ok {
		return q, nil
	}
	if formatIDPattern.MatchString(q) {
		return q, nil
	}
	return "", errors.New("invalid quality")
}

// FormatArgs is the yt-dlp format selection for a job. The caller must pass
// these as separate arguments, never through a shell.
func FormatArgs(kind, quality string) ([]string, error) {
	q, err := NormalizeQuality(kind, quality)
	if err != nil {
		return nil, err
	}
	if kind == "audio" {
		return []string{"-f", "bestaudio/best", "-x", "--audio-format", "m4a", "--audio-quality", "0"}, nil
	}
	if q == "best" {
		return []string{"-f", "bv*+ba/b", "--merge-output-format", "mp4"}, nil
	}
	if q == QualityCar {
		return []string{
			"-f", carFormat,
			"--merge-output-format", "mp4",
			"--postprocessor-args", carMerge,
		}, nil
	}
	if _, ok := standardHeights[q]; ok {
		sel := fmt.Sprintf("bv*[height<=%s]+ba/b[height<=%s]", q, q)
		return []string{"-f", sel, "--merge-output-format", "mp4"}, nil
	}
	return []string{"-f", q, "--merge-output-format", "mp4"}, nil
}

// Format is one yt-dlp format entry returned to clients.
type Format struct {
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	Resolution     string  `json:"resolution,omitempty"`
	Width          int     `json:"width,omitempty"`
	Height         int     `json:"height,omitempty"`
	FPS            float64 `json:"fps,omitempty"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox bool    `json:"filesize_approx"`
	VCodec         string  `json:"vcodec,omitempty"`
	ACodec         string  `json:"acodec,omitempty"`
	TBR            float64 `json:"tbr,omitempty"`
	Note           string  `json:"note,omitempty"`
	HasVideo       bool    `json:"has_video"`
	HasAudio       bool    `json:"has_audio"`
}

// Quality is a height choice with an estimated size (video plus best audio
// when the selected video format has no audio).
type Quality struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Height         int    `json:"height"`
	Ext            string `json:"ext,omitempty"`
	Filesize       int64  `json:"filesize"`
	FilesizeApprox bool   `json:"filesize_approx"`
}

// Probe is the formats response.
type Probe struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Thumbnail  string    `json:"thumbnail,omitempty"`
	Duration   float64   `json:"duration,omitempty"`
	Channel    string    `json:"channel,omitempty"`
	Formats    []Format  `json:"formats"`
	Qualities  []Quality `json:"qualities"`
	WebpageURL string    `json:"webpage_url,omitempty"`
}

type rawDump struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Thumbnail  string      `json:"thumbnail"`
	Duration   float64     `json:"duration"`
	Channel    string      `json:"channel"`
	Uploader   string      `json:"uploader"`
	WebpageURL string      `json:"webpage_url"`
	Formats    []rawFormat `json:"formats"`
}

type rawFormat struct {
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	Resolution     string  `json:"resolution"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FPS            float64 `json:"fps"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	TBR            float64 `json:"tbr"`
	FormatNote     string  `json:"format_note"`
}

func probeFromDump(d rawDump) *Probe {
	formats := make([]Format, 0, len(d.Formats))
	var bestAudio int64
	audioApprox := false
	for _, rf := range d.Formats {
		if rf.FormatID == "" {
			continue
		}
		ext := strings.ToLower(rf.Ext)
		if ext == "mhtml" || ext == "jpg" || ext == "png" || ext == "webp" {
			continue
		}
		hasV := rf.VCodec != "" && rf.VCodec != "none"
		hasA := rf.ACodec != "" && rf.ACodec != "none"
		if !hasV && !hasA {
			continue
		}
		size := rf.Filesize
		approx := false
		if size <= 0 && rf.FilesizeApprox > 0 {
			size = rf.FilesizeApprox
			approx = true
		}
		f := Format{
			FormatID:       rf.FormatID,
			Ext:            rf.Ext,
			Resolution:     rf.Resolution,
			Width:          rf.Width,
			Height:         rf.Height,
			FPS:            rf.FPS,
			Filesize:       size,
			FilesizeApprox: approx,
			VCodec:         rf.VCodec,
			ACodec:         rf.ACodec,
			TBR:            rf.TBR,
			Note:           rf.FormatNote,
			HasVideo:       hasV,
			HasAudio:       hasA,
		}
		formats = append(formats, f)
		if hasA && !hasV && size > bestAudio {
			bestAudio = size
			audioApprox = approx
		}
	}
	sort.SliceStable(formats, func(i, j int) bool {
		if formats[i].Height == formats[j].Height {
			return formats[i].TBR > formats[j].TBR
		}
		return formats[i].Height > formats[j].Height
	})

	type best struct {
		q     Quality
		score float64
		mp4   bool
	}
	byHeight := map[int]best{}
	for _, f := range formats {
		if !f.HasVideo || f.Height <= 0 {
			continue
		}
		size := f.Filesize
		approx := f.FilesizeApprox
		if !f.HasAudio && bestAudio > 0 && size > 0 {
			size += bestAudio
			approx = approx || audioApprox
		}
		mp4 := strings.EqualFold(f.Ext, "mp4")
		cur, ok := byHeight[f.Height]
		if ok && (f.TBR < cur.score || (f.TBR == cur.score && !mp4)) {
			continue
		}
		label := strconv.Itoa(f.Height) + "p"
		if f.FPS >= 50 {
			label += strconv.Itoa(int(f.FPS + 0.5))
		}
		ext := f.Ext
		if !f.HasAudio {
			ext = "mp4"
		}
		byHeight[f.Height] = best{
			score: f.TBR,
			mp4:   mp4,
			q: Quality{
				ID:             strconv.Itoa(f.Height),
				Label:          label,
				Height:         f.Height,
				Ext:            ext,
				Filesize:       size,
				FilesizeApprox: approx,
			},
		}
	}
	qualities := make([]Quality, 0, len(byHeight))
	for _, b := range byHeight {
		// Only advertise heights the downloader treats as a height cap.
		// Other heights remain available as format ids in Formats.
		if _, ok := standardHeights[b.q.ID]; !ok {
			continue
		}
		qualities = append(qualities, b.q)
	}
	sort.Slice(qualities, func(i, j int) bool { return qualities[i].Height > qualities[j].Height })

	channel := d.Channel
	if channel == "" {
		channel = d.Uploader
	}
	if formats == nil {
		formats = []Format{}
	}
	return &Probe{
		ID:         d.ID,
		Title:      d.Title,
		Thumbnail:  d.Thumbnail,
		Duration:   d.Duration,
		Channel:    channel,
		Formats:    formats,
		Qualities:  qualities,
		WebpageURL: d.WebpageURL,
	}
}
