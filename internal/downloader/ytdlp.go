package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// YTDLP runs the yt-dlp binary.
//
// Resume: every attempt uses the same TempDir with --continue. Pause and
// shutdown send SIGINT to the process group so yt-dlp keeps its .part files,
// then SIGKILL if it does not exit. The queue moves a finished file out of
// the temp directory only after a clean exit.
type YTDLP struct {
	bin       string
	ffmpeg    string
	jsRuntime string
	cookies   string

	mu           sync.Mutex
	noJSRuntimes bool
	proxy        string
}

func New(bin, ffmpeg, jsRuntime, cookies string) *YTDLP {
	if strings.TrimSpace(bin) == "" {
		bin = "yt-dlp"
	}
	return &YTDLP{bin: bin, ffmpeg: ffmpeg, jsRuntime: jsRuntime, cookies: cookies}
}

func (y *YTDLP) Available() bool {
	_, err := exec.LookPath(y.bin)
	return err == nil
}

// SetProxy sets the proxy used when yt-dlp fetches or downloads a video.
// An empty string connects directly. Version checks and yt-dlp -U do not use it.
func (y *YTDLP) SetProxy(proxy string) {
	y.mu.Lock()
	y.proxy = strings.TrimSpace(proxy)
	y.mu.Unlock()
}

func (y *YTDLP) currentProxy() string {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.proxy
}

func (y *YTDLP) base() []string {
	args := []string{"--no-playlist", "--no-colors"}
	if y.ffmpeg != "" {
		args = append(args, "--ffmpeg-location", y.ffmpeg)
	}
	if y.passJSRuntimes() {
		args = append(args, "--js-runtimes", y.jsRuntime)
	}
	if y.cookies != "" {
		args = append(args, "--cookies", y.cookies)
	}
	if proxy := y.currentProxy(); proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	return args
}

func (y *YTDLP) Probe(ctx context.Context, pageURL string) (*Probe, error) {
	out, err := y.probe(ctx, pageURL)
	if y.dropUnsupportedJS(err) {
		out, err = y.probe(ctx, pageURL)
	}
	if err != nil {
		return nil, commandError(err)
	}
	var dump rawDump
	if err := json.Unmarshal(out, &dump); err != nil {
		return nil, fmt.Errorf("yt-dlp json: %w", err)
	}
	return probeFromDump(dump), nil
}

func (y *YTDLP) Version(ctx context.Context) (string, error) {
	// The standalone binary is a stub that unpacks an interpreter. A cold
	// start can sit in dyld for half a minute before printing the version.
	out, err := y.exec(ctx, 60*time.Second, false, "--version")
	if err != nil {
		return "", commandError(err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (y *YTDLP) Update(ctx context.Context) (string, error) {
	out, err := y.exec(ctx, 3*time.Minute, true, "-U")
	text := tailText(string(out), 4000)
	if err != nil {
		if text == "" {
			return "", commandError(err)
		}
		return text, fmt.Errorf("%s", text)
	}
	return text, nil
}

func (y *YTDLP) Download(ctx context.Context, req Request, cb func(Progress)) (string, error) {
	// Keep this OS thread alive so Linux Pdeathsig is attached to a thread
	// that dies with the server process.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if title, id := y.metadata(ctx, req.URL); (title != "" || id != "") && cb != nil {
		cb(Progress{Title: title, VideoID: id})
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(req.TempDir, 0o755); err != nil {
		return "", err
	}
	formatArgs, err := FormatArgs(req.Kind, req.Quality)
	if err != nil {
		return "", err
	}
	args := append(y.base(), formatArgs...)
	args = append(args,
		"--newline",
		"--progress",
		"--continue",
		"--no-overwrites",
		"--paths", "home:"+req.TempDir,
		"--paths", "temp:"+req.TempDir,
		"-o", "%(title).180B [%(id)s].%(ext)s",
		req.URL,
	)
	cmd := exec.Command(y.bin, args...)
	cmd.Dir = req.TempDir
	configureCommand(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			signalProcess(cmd, false)
			select {
			case <-stop:
			case <-time.After(1500 * time.Millisecond):
				signalProcess(cmd, true)
			}
		case <-stop:
		}
	}()

	var mu sync.Mutex
	var dest string
	var errLines []string
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			ev, ok := ParseLine(line)
			if !ok {
				continue
			}
			switch ev.Kind {
			case "destination":
				if UsableFile(ev.Path) || !strings.HasSuffix(strings.ToLower(ev.Path), ".part") {
					mu.Lock()
					dest = ev.Path
					mu.Unlock()
				}
			case "progress":
				if cb == nil {
					continue
				}
				downloaded := int64(0)
				if ev.TotalBytes > 0 {
					downloaded = int64(ev.Percent / 100 * float64(ev.TotalBytes))
				}
				cb(Progress{
					HasPercent:      true,
					Percent:         ev.Percent,
					BytesDownloaded: downloaded,
					TotalBytes:      ev.TotalBytes,
					SpeedBPS:        ev.SpeedBPS,
					ETA:             ev.ETA,
				})
			case "error":
				mu.Lock()
				if len(errLines) < 30 && ev.Text != "" {
					errLines = append(errLines, ev.Text)
				}
				mu.Unlock()
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); scan(stdout) }()
	go func() { defer wg.Done(); scan(stderr) }()
	waitErr := cmd.Wait()
	close(stop)
	wg.Wait()

	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if waitErr != nil {
		mu.Lock()
		msg := strings.Join(errLines, " | ")
		mu.Unlock()
		if msg == "" {
			msg = commandError(waitErr).Error()
		}
		return "", errors.New(tailText(msg, 500))
	}

	mu.Lock()
	chosen := dest
	mu.Unlock()
	if UsableFile(chosen) {
		return chosen, nil
	}
	return FindOutput(req.TempDir)
}

func (y *YTDLP) probe(ctx context.Context, pageURL string) ([]byte, error) {
	args := append(y.base(), "-J", "--skip-download", "--no-warnings", pageURL)
	return y.exec(ctx, 90*time.Second, false, args...)
}

func (y *YTDLP) metadata(ctx context.Context, pageURL string) (title, id string) {
	args := append(y.base(), "--skip-download", "--no-warnings", "--print", "%(title)s", "--print", "%(id)s", pageURL)
	out, err := y.exec(ctx, 45*time.Second, false, args...)
	if y.dropUnsupportedJS(err) {
		args = append(y.base(), "--skip-download", "--no-warnings", "--print", "%(title)s", "--print", "%(id)s", pageURL)
		out, err = y.exec(ctx, 45*time.Second, false, args...)
	}
	if err != nil {
		return "", ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", ""
	}
	title = strings.TrimSpace(lines[0])
	if len(lines) >= 2 {
		id = strings.TrimSpace(lines[len(lines)-1])
	}
	return title, id
}

// exec runs yt-dlp in its own process group. CommandContext.Kill only
// signals the direct child, and the standalone binary's parent is a stub
// waiting on the real interpreter. A timeout has to signal the group, then
// stop waiting on pipes the child inherited.
func (y *YTDLP) exec(ctx context.Context, timeout time.Duration, combined bool, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, y.bin, args...)
	configureCommand(cmd)
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error {
		signalProcess(cmd, true)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil
	}
	var out []byte
	var err error
	if combined {
		out, err = cmd.CombinedOutput()
	} else {
		out, err = cmd.Output()
	}
	if err != nil && ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, err
}

// Older pip builds of yt-dlp reject --js-runtimes. The first call records
// that and returns true so the caller can run the same command once more.
func (y *YTDLP) dropUnsupportedJS(err error) bool {
	if !unsupportedJSRuntimes(err) {
		return false
	}
	y.mu.Lock()
	already := y.noJSRuntimes
	y.noJSRuntimes = true
	y.mu.Unlock()
	if !already {
		slog.Warn("yt-dlp does not support --js-runtimes; continuing without a JavaScript runtime", "bin", y.bin)
	}
	return !already
}

func (y *YTDLP) passJSRuntimes() bool {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.jsRuntime != "" && !y.noJSRuntimes
}

func unsupportedJSRuntimes(err error) bool {
	return strings.Contains(errText(err), "no such option: --js-runtimes")
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return string(ee.Stderr)
	}
	return err.Error()
}

func commandError(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg := cleanYTDLPOutput(string(ee.Stderr))
		if msg != "" {
			return errors.New(tailText(msg, 500))
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("yt-dlp timed out")
	}
	return err
}

// cleanYTDLPOutput drops Python warning noise so the extension shows the
// yt-dlp error itself.
func cleanYTDLPOutput(s string) string {
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || noiseLine(line) {
			continue
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

func noiseLine(line string) bool {
	if strings.Contains(line, "NotOpenSSLWarning") || strings.Contains(line, "warnings.warn") {
		return true
	}
	if strings.Contains(line, "site-packages/urllib3") {
		return true
	}
	if strings.HasPrefix(line, "Deprecated Feature:") {
		return true
	}
	return strings.HasPrefix(line, "Usage: yt-dlp")
}

func tailText(s string, n int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
