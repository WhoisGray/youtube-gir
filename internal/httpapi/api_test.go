package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/WhoisGray/youtube-gir/internal/config"
	"github.com/WhoisGray/youtube-gir/internal/downloader"
	"github.com/WhoisGray/youtube-gir/internal/queue"
	"github.com/WhoisGray/youtube-gir/internal/store"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakeRunner struct {
	probe   *downloader.Probe
	started chan string
	release chan struct{}
	part    bool
	proxy   string
}

func (f *fakeRunner) Probe(context.Context, string) (*downloader.Probe, error) {
	if f.probe == nil {
		return &downloader.Probe{ID: "abcdefghijk", Title: "Probed", Formats: []downloader.Format{}, Qualities: []downloader.Quality{}}, nil
	}
	return f.probe, nil
}

func (f *fakeRunner) Download(ctx context.Context, req downloader.Request, cb func(downloader.Progress)) (string, error) {
	if err := os.MkdirAll(req.TempDir, 0o755); err != nil {
		return "", err
	}
	if f.part {
		if err := os.WriteFile(filepath.Join(req.TempDir, "clip.mp4.part"), []byte("partial"), 0o644); err != nil {
			return "", err
		}
	}
	if cb != nil {
		cb(downloader.Progress{Title: "Test Title", VideoID: "abcdefghijk"})
	}
	if f.started != nil {
		f.started <- req.ID
	}
	if f.release != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-f.release:
		}
	}
	path := filepath.Join(req.TempDir, "clip.mp4")
	if err := os.WriteFile(path, []byte("video-bytes"), 0o644); err != nil {
		return "", err
	}
	if cb != nil {
		cb(downloader.Progress{HasPercent: true, Percent: 100, TotalBytes: 11, BytesDownloaded: 11})
	}
	return path, nil
}

func (f *fakeRunner) SetProxy(proxy string)                   { f.proxy = proxy }
func (f *fakeRunner) Version(context.Context) (string, error) { return "test", nil }
func (f *fakeRunner) Update(context.Context) (string, error)  { return "updated", nil }
func (f *fakeRunner) Available() bool                         { return true }

type fixture struct {
	srv     *httptest.Server
	store   *store.Store
	queue   *queue.Manager
	key     string
	tempDir string
}

func newFixture(t *testing.T, runner downloader.Runner) *fixture {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		AdminKey:      "test-admin-key-0123456789",
		DataDir:       dir,
		DBPath:        filepath.Join(dir, "db", "t.sqlite"),
		DownloadDir:   filepath.Join(dir, "downloads"),
		TempDir:       filepath.Join(dir, "tmp"),
		MaxConcurrent: 2,
		YTDLPPath:     "yt-dlp",
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureAdmin(context.Background(), cfg.AdminKey); err != nil {
		t.Fatal(err)
	}
	mgr := queue.New(st, runner, cfg)
	srv := httptest.NewServer(Router(New(st, mgr, runner, cfg)))
	t.Cleanup(func() {
		srv.Close()
		mgr.Shutdown()
		st.Close()
	})
	return &fixture{srv: srv, store: st, queue: mgr, key: cfg.AdminKey, tempDir: cfg.TempDir}
}

func (f *fixture) do(t *testing.T, method, path, key string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, f.srv.URL+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decode(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	defer res.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func waitStatus(t *testing.T, st *store.Store, id, want string) *store.Download {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	var last *store.Download
	for time.Now().Before(deadline) {
		d, err := st.GetDownload(context.Background(), id)
		if err == nil {
			last = d
			if d.Status == want {
				return d
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if last != nil {
		t.Fatalf("status %s, want %s (err %s)", last.Status, want, last.Error)
	}
	t.Fatalf("missing download, want %s", want)
	return nil
}

func TestHealthAndAuth(t *testing.T) {
	f := newFixture(t, &fakeRunner{})
	res, err := http.Get(f.srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	body := decode(t, res)
	if res.StatusCode != 200 || body["status"] != "ok" || body["ytdlp"] != true {
		t.Fatalf("%d %+v", res.StatusCode, body)
	}
	res = f.do(t, http.MethodGet, "/api/v1/me", "", nil)
	if res.StatusCode != 401 {
		t.Fatalf("status %d", res.StatusCode)
	}
	res.Body.Close()
	res = f.do(t, http.MethodGet, "/api/v1/me", f.key, nil)
	body = decode(t, res)
	if res.StatusCode != 200 || body["role"] != "admin" {
		t.Fatalf("%d %+v", res.StatusCode, body)
	}
}

func TestQueueIsolationAndFile(t *testing.T) {
	f := newFixture(t, &fakeRunner{})
	res := f.do(t, http.MethodPost, "/api/v1/users", f.key, map[string]string{"username": "alice"})
	created := decode(t, res)
	if res.StatusCode != 201 {
		t.Fatalf("create user %d %+v", res.StatusCode, created)
	}
	alice := created["token"].(string)
	res = f.do(t, http.MethodPost, "/api/v1/users", f.key, map[string]string{"username": "bob"})
	bobBody := decode(t, res)
	bob := bobBody["token"].(string)

	res = f.do(t, http.MethodPost, "/api/v1/downloads", alice, map[string]string{
		"url":     "https://youtu.be/abcdefghijk?t=3",
		"kind":    "video",
		"quality": "1080",
		"title":   "From the page",
	})
	queued := decode(t, res)
	if res.StatusCode != 202 {
		t.Fatalf("queue %d %+v", res.StatusCode, queued)
	}
	dl := queued["download"].(map[string]any)
	id := dl["id"].(string)
	token := dl["file_token"].(string)
	done := waitStatus(t, f.store, id, store.StatusCompleted)
	if done.Title != "Test Title" || done.URL != "https://www.youtube.com/watch?v=abcdefghijk" || done.FileSize != 11 {
		t.Fatalf("%+v", done)
	}

	res, err := http.Get(f.srv.URL + "/api/v1/files/" + token)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || buf.String() != "video-bytes" {
		t.Fatalf("file %d %q", res.StatusCode, buf.String())
	}

	res = f.do(t, http.MethodGet, "/api/v1/downloads", bob, nil)
	list := decode(t, res)
	items := list["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("bob sees %v", items)
	}
	res = f.do(t, http.MethodGet, "/api/v1/downloads/"+id, bob, nil)
	if res.StatusCode != 404 {
		t.Fatalf("bob get %d", res.StatusCode)
	}
	res.Body.Close()

	res = f.do(t, http.MethodGet, "/api/v1/downloads?scope=all", f.key, nil)
	all := decode(t, res)
	if len(all["items"].([]any)) != 1 {
		t.Fatalf("admin list %+v", all)
	}
	res = f.do(t, http.MethodPost, "/api/v1/downloads", alice, map[string]string{
		"url":  "https://example.com/watch?v=abcdefghijk",
		"kind": "video",
	})
	if res.StatusCode != 400 {
		t.Fatalf("bad url %d", res.StatusCode)
	}
	res.Body.Close()

	res = f.do(t, http.MethodPost, "/api/v1/users", alice, map[string]string{"username": "carol"})
	if res.StatusCode != 403 {
		t.Fatalf("user create key %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestPauseCancelAndDuplicate(t *testing.T) {
	started := make(chan string, 1)
	f := newFixture(t, &fakeRunner{started: started, release: make(chan struct{}), part: true})
	res := f.do(t, http.MethodPost, "/api/v1/downloads", f.key, map[string]string{
		"url":  "https://www.youtube.com/watch?v=abcdefghijk",
		"kind": "audio",
	})
	body := decode(t, res)
	if res.StatusCode != 202 {
		t.Fatalf("%d %+v", res.StatusCode, body)
	}
	id := body["download"].(map[string]any)["id"].(string)
	select {
	case got := <-started:
		if got != id {
			t.Fatal(got)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("download did not start")
	}

	res = f.do(t, http.MethodPost, "/api/v1/downloads", f.key, map[string]string{
		"url":  "https://www.youtube.com/shorts/abcdefghijk",
		"kind": "video",
	})
	// Different kind, so this is a second job. It should queue, not duplicate.
	other := decode(t, res)
	if res.StatusCode != 202 || other["duplicate"] == true {
		t.Fatalf("second kind %+v", other)
	}

	res = f.do(t, http.MethodPost, "/api/v1/downloads", f.key, map[string]string{
		"url":  "https://youtu.be/abcdefghijk",
		"kind": "audio",
	})
	dup := decode(t, res)
	if res.StatusCode != 200 || dup["duplicate"] != true {
		t.Fatalf("duplicate %+v status %d", dup, res.StatusCode)
	}
	if dup["download"].(map[string]any)["id"] != id {
		t.Fatal("duplicate id")
	}

	res = f.do(t, http.MethodPost, "/api/v1/downloads/"+id+"/pause", f.key, map[string]string{})
	paused := decode(t, res)
	if res.StatusCode != 200 || paused["download"].(map[string]any)["status"] != store.StatusPaused {
		t.Fatalf("pause %+v", paused)
	}
	part := filepath.Join(f.tempDir, id, "clip.mp4.part")
	if _, err := os.Stat(part); err != nil {
		t.Fatal("part file should remain after pause", err)
	}

	res = f.do(t, http.MethodPost, "/api/v1/downloads/"+id+"/cancel", f.key, map[string]string{})
	cancelled := decode(t, res)
	if res.StatusCode != 200 || cancelled["download"].(map[string]any)["status"] != store.StatusCancelled {
		t.Fatalf("cancel %+v", cancelled)
	}
	if _, err := os.Stat(filepath.Join(f.tempDir, id)); !os.IsNotExist(err) {
		t.Fatal("temp dir should be removed", err)
	}
}

func TestDownloadProxyAdminOnly(t *testing.T) {
	runner := &fakeRunner{}
	f := newFixture(t, runner)
	res := f.do(t, http.MethodPost, "/api/v1/users", f.key, map[string]string{"username": "alice"})
	created := decode(t, res)
	if res.StatusCode != 201 {
		t.Fatalf("create user %d %+v", res.StatusCode, created)
	}
	alice := created["token"].(string)

	res = f.do(t, http.MethodGet, "/api/v1/system/proxy", alice, nil)
	if res.StatusCode != 403 {
		t.Fatalf("user get %d", res.StatusCode)
	}
	res.Body.Close()

	res = f.do(t, http.MethodGet, "/api/v1/system/proxy", f.key, nil)
	body := decode(t, res)
	if res.StatusCode != 200 || body["proxy"] != "" {
		t.Fatalf("empty %+v", body)
	}

	res = f.do(t, http.MethodPut, "/api/v1/system/proxy", f.key, map[string]string{"proxy": "socks5://10.1.2.3:1080"})
	body = decode(t, res)
	if res.StatusCode != 200 || body["proxy"] != "socks5://10.1.2.3:1080" {
		t.Fatalf("set %+v status %d", body, res.StatusCode)
	}
	if runner.proxy != "socks5://10.1.2.3:1080" {
		t.Fatalf("runner proxy %q", runner.proxy)
	}
	stored, ok, err := f.store.GetSetting(context.Background(), store.SettingDownloadProxy)
	if err != nil || !ok || stored != "socks5://10.1.2.3:1080" {
		t.Fatalf("stored %q %v %v", stored, ok, err)
	}

	res = f.do(t, http.MethodPut, "/api/v1/system/proxy", f.key, map[string]string{"proxy": "ftp://127.0.0.1:1080"})
	if res.StatusCode != 400 {
		t.Fatalf("bad scheme %d", res.StatusCode)
	}
	res.Body.Close()
	if runner.proxy != "socks5://10.1.2.3:1080" {
		t.Fatal("invalid proxy replaced the saved one")
	}

	res = f.do(t, http.MethodPut, "/api/v1/system/proxy", alice, map[string]string{"proxy": "socks5://127.0.0.1:1"})
	if res.StatusCode != 403 {
		t.Fatalf("user put %d", res.StatusCode)
	}
	res.Body.Close()

	res = f.do(t, http.MethodPut, "/api/v1/system/proxy", f.key, map[string]string{"proxy": ""})
	body = decode(t, res)
	if res.StatusCode != 200 || body["proxy"] != "" || runner.proxy != "" {
		t.Fatalf("clear %+v runner %q", body, runner.proxy)
	}
}
