// Package queue runs a bounded pool of yt-dlp jobs.
package queue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/WhoisGray/youtube-gir/internal/config"
	"github.com/WhoisGray/youtube-gir/internal/downloader"
	"github.com/WhoisGray/youtube-gir/internal/store"
)

const (
	intentRun = iota
	intentPause
	intentCancel
	intentShutdown
)

type job struct {
	cancel context.CancelFunc
	intent int
	done   chan struct{}
}

// Manager claims queued rows and downloads them.
// On startup, rows left in "downloading" by a crash go back to the queue.
// The temp directory does not change, so yt-dlp continues from .part files.
type Manager struct {
	store   *store.Store
	runner  downloader.Runner
	tempDir string
	libDir  string
	max     int

	mu     sync.Mutex
	jobs   map[string]*job
	last   map[string]time.Time
	shown  map[string]float64
	closed bool
	stop   chan struct{}
	once   sync.Once
}

func New(st *store.Store, runner downloader.Runner, cfg config.Config) *Manager {
	max := cfg.MaxConcurrent
	if max < 1 {
		max = 1
	}
	m := &Manager{
		store:   st,
		runner:  runner,
		tempDir: cfg.TempDir,
		libDir:  cfg.DownloadDir,
		max:     max,
		jobs:    map[string]*job{},
		last:    map[string]time.Time{},
		shown:   map[string]float64{},
		stop:    make(chan struct{}),
	}
	n, err := st.RecoverInterrupted(context.Background())
	if err != nil {
		slog.Error("recover interrupted downloads", "err", err)
	} else if n > 0 {
		slog.Info("requeued interrupted downloads", "count", n)
	}
	go m.loop()
	return m
}

func (m *Manager) loop() {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.pump()
		}
	}
}

func (m *Manager) pump() {
	for {
		m.mu.Lock()
		full := m.closed || len(m.jobs) >= m.max
		m.mu.Unlock()
		if full {
			return
		}
		d, err := m.store.ClaimNext(context.Background())
		if err != nil {
			slog.Error("claim download", "err", err)
			return
		}
		if d == nil {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		j := &job{cancel: cancel, intent: intentRun, done: make(chan struct{})}
		m.mu.Lock()
		if m.closed || len(m.jobs) >= m.max {
			m.mu.Unlock()
			cancel()
			_ = m.store.SetStatus(context.Background(), d.ID, store.StatusQueued, "", []string{store.StatusDownloading})
			return
		}
		m.jobs[d.ID] = j
		m.mu.Unlock()
		go m.run(ctx, j, *d)
	}
}

func (m *Manager) run(ctx context.Context, j *job, d store.Download) {
	defer func() {
		j.cancel()
		m.mu.Lock()
		delete(m.jobs, d.ID)
		delete(m.last, d.ID)
		delete(m.shown, d.ID)
		m.mu.Unlock()
		close(j.done)
	}()

	cur, err := m.store.GetDownload(context.Background(), d.ID)
	if err != nil || cur.Status != store.StatusDownloading {
		return
	}
	if cur.Progress > 0 {
		m.mu.Lock()
		m.shown[d.ID] = advancePercent(0, cur.Progress)
		m.mu.Unlock()
	}
	slog.Info("download started", "id", d.ID, "kind", d.Kind, "quality", d.Quality)

	temp := filepath.Join(m.tempDir, d.ID)
	out, err := m.runner.Download(ctx, downloader.Request{
		ID:      d.ID,
		URL:     d.URL,
		Kind:    d.Kind,
		Quality: d.Quality,
		TempDir: temp,
	}, func(p downloader.Progress) {
		m.onProgress(d.ID, p)
	})

	m.mu.Lock()
	intent := j.intent
	m.mu.Unlock()

	switch intent {
	case intentShutdown:
		slog.Info("download stopped for shutdown", "id", d.ID)
		return
	case intentPause:
		if err := m.store.SetStatus(context.Background(), d.ID, store.StatusPaused, "", []string{store.StatusDownloading}); err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Error("mark paused", "id", d.ID, "err", err)
		}
		return
	case intentCancel:
		if rmErr := os.RemoveAll(temp); rmErr != nil {
			slog.Warn("temp cleanup", "id", d.ID, "err", rmErr)
		}
		if err := m.store.SetStatus(context.Background(), d.ID, store.StatusCancelled, "", []string{store.StatusDownloading, store.StatusPaused, store.StatusQueued}); err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Error("mark cancelled", "id", d.ID, "err", err)
		}
		return
	}

	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		msg := err.Error()
		if utf8.RuneCountInString(msg) > 500 {
			r := []rune(msg)
			msg = string(r[len(r)-500:])
		}
		slog.Error("download failed", "id", d.ID, "err", msg)
		_ = m.store.SetStatus(context.Background(), d.ID, store.StatusFailed, msg, []string{store.StatusDownloading})
		return
	}

	finalPath, name, size, pubErr := m.publish(d.ID, d.UserID, out, temp)
	if pubErr != nil {
		slog.Error("publish download", "id", d.ID, "err", pubErr)
		_ = m.store.SetStatus(context.Background(), d.ID, store.StatusFailed, pubErr.Error(), []string{store.StatusDownloading})
		return
	}
	_ = os.RemoveAll(temp)
	if err := m.store.MarkCompleted(context.Background(), d.ID, finalPath, name, size); err != nil {
		slog.Error("mark completed", "id", d.ID, "err", err)
		return
	}
	slog.Info("download finished", "id", d.ID, "bytes", size)
}

func (m *Manager) onProgress(id string, p downloader.Progress) {
	if p.Title != "" || p.VideoID != "" {
		if err := m.store.UpdateMeta(context.Background(), id, p.Title, p.VideoID); err != nil {
			slog.Debug("update meta", "id", id, "err", err)
		}
	}
	if !p.HasPercent {
		return
	}
	now := time.Now()
	m.mu.Lock()
	shown := advancePercent(m.shown[id], p.Percent)
	m.shown[id] = shown
	last := m.last[id]
	if p.Percent < 100 && now.Sub(last) < 400*time.Millisecond {
		m.mu.Unlock()
		return
	}
	m.last[id] = now
	m.mu.Unlock()
	if err := m.store.UpdateProgress(context.Background(), id, store.ProgressUpdate{
		Percent:         shown,
		BytesDownloaded: p.BytesDownloaded,
		TotalBytes:      p.TotalBytes,
		SpeedBPS:        p.SpeedBPS,
		ETA:             p.ETA,
	}); err != nil {
		slog.Debug("update progress", "id", id, "err", err)
	}
}

// advancePercent keeps the bar from snapping backward when yt-dlp finishes
// one stream and starts the next. The row stays under 100 until it is
// marked completed.
func advancePercent(shown, raw float64) float64 {
	if math.IsNaN(shown) || shown < 0 {
		shown = 0
	}
	if math.IsNaN(raw) || raw < 0 {
		raw = 0
	}
	if raw > 100 {
		raw = 100
	}
	if raw > shown {
		shown = raw
	}
	if shown > 99 {
		return 99
	}
	return shown
}

func (m *Manager) publish(id string, userID int64, src, temp string) (string, string, int64, error) {
	if !downloader.UsableFile(src) {
		found, err := downloader.FindOutput(temp)
		if err != nil {
			return "", "", 0, err
		}
		src = found
	}
	ext := filepath.Ext(src)
	dir := filepath.Join(m.libDir, strconv.FormatInt(userID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", 0, err
	}
	dest := filepath.Join(dir, id+ext)
	if err := moveFile(src, dest); err != nil {
		return "", "", 0, err
	}
	info, err := os.Stat(dest)
	if err != nil {
		return "", "", 0, err
	}
	title := id
	if cur, err := m.store.GetDownload(context.Background(), id); err == nil && strings.TrimSpace(cur.Title) != "" {
		title = cur.Title
	}
	return dest, FileName(title, ext), info.Size(), nil
}

// Pause stops a running download and keeps the temp files.
// A queued item is marked paused so the worker will not claim it.
func (m *Manager) Pause(ctx context.Context, id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	if j != nil {
		j.intent = intentPause
		j.cancel()
		done := j.done
		m.mu.Unlock()
		return waitDone(ctx, done)
	}
	m.mu.Unlock()
	return m.store.SetStatus(ctx, id, store.StatusPaused, "", []string{store.StatusQueued, store.StatusDownloading})
}

// Cancel stops a download, deletes its temp files, and marks it cancelled.
func (m *Manager) Cancel(ctx context.Context, id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	if j != nil {
		j.intent = intentCancel
		j.cancel()
		done := j.done
		m.mu.Unlock()
		return waitDone(ctx, done)
	}
	m.mu.Unlock()
	if err := os.RemoveAll(filepath.Join(m.tempDir, id)); err != nil {
		slog.Warn("temp cleanup", "id", id, "err", err)
	}
	return m.store.SetStatus(ctx, id, store.StatusCancelled, "", []string{
		store.StatusQueued, store.StatusDownloading, store.StatusPaused, store.StatusFailed,
	})
}

// Shutdown stops new claims and interrupts running processes.
// Those rows stay "downloading" so the next process resumes them.
func (m *Manager) Shutdown() {
	m.once.Do(func() { close(m.stop) })
	m.mu.Lock()
	m.closed = true
	jobs := make([]*job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j.intent == intentRun {
			j.intent = intentShutdown
		}
		j.cancel()
		jobs = append(jobs, j)
	}
	m.mu.Unlock()
	deadline := time.After(8 * time.Second)
	for _, j := range jobs {
		select {
		case <-j.done:
		case <-deadline:
			return
		}
	}
}

func waitDone(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// moveFile renames, and copies when the temp and library volumes differ.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".partcopy"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(src)
}

// FileName is the name shown to the browser when it saves a finished file.
func FileName(title, ext string) string {
	title = strings.TrimSpace(title)
	title = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, title)
	title = strings.Trim(title, " .")
	runes := []rune(title)
	if len(runes) > 120 {
		title = string(runes[:120])
	}
	if title == "" {
		title = "download"
	}
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "" {
		ext = "bin"
	}
	return title + "." + ext
}
