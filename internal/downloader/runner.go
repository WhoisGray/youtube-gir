package downloader

import "context"

// Request is one queued download. TempDir is stable across retries so
// yt-dlp can continue .part files after a crash, pause, or dropped network.
type Request struct {
	ID      string
	URL     string
	Kind    string
	Quality string
	TempDir string
}

// Progress is a metadata or percentage update.
type Progress struct {
	HasPercent      bool
	Percent         float64
	BytesDownloaded int64
	TotalBytes      int64
	SpeedBPS        int64
	ETA             string
	Title           string
	VideoID         string
}

// Runner downloads and inspects videos. Tests substitute a fake.
type Runner interface {
	Probe(ctx context.Context, pageURL string) (*Probe, error)
	Download(ctx context.Context, req Request, cb func(Progress)) (string, error)
	Version(ctx context.Context) (string, error)
	Update(ctx context.Context) (string, error)
	Available() bool
}
