package gui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	if os.Getenv("YG_LOCK_HELPER") == "1" {
		os.Exit(holdLockUntilStdin())
	}
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func holdLockUntilStdin() int {
	dir := os.Getenv("YG_LOCK_DIR")
	deadline := time.Now().Add(5 * time.Second)
	for {
		release, already, err := ClaimRunning(dir)
		if err != nil {
			return 2
		}
		if !already {
			buf := make([]byte, 1)
			_, _ = os.Stdin.Read(buf)
			release()
			return 0
		}
		if time.Now().After(deadline) {
			return 3
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPageKeyVisibility(t *testing.T) {
	const key = "gui-test-key-0123456789abcdef"
	r := mount(key, t.TempDir(), func() {}, func(string) error { return nil })

	remote := get(r, http.MethodGet, "/app/", "10.1.1.1:1234", "")
	if remote.Code != http.StatusOK {
		t.Fatalf("remote status %d", remote.Code)
	}
	if strings.Contains(remote.Body.String(), key) {
		t.Fatal("remote page included the admin key")
	}
	if !strings.Contains(remote.Body.String(), `"key":""`) {
		t.Fatal("remote page did not hide the key")
	}

	blank := get(r, http.MethodGet, "/app/", "", "")
	if strings.Contains(blank.Body.String(), key) {
		t.Fatal("page with no client address included the admin key")
	}

	local := get(r, http.MethodGet, "/app/", "127.0.0.1:4321", "")
	if local.Code != http.StatusOK {
		t.Fatalf("local status %d", local.Code)
	}
	if !strings.Contains(local.Body.String(), key) {
		t.Fatal("loopback page omitted the admin key")
	}
	if !strings.Contains(local.Body.String(), `id="url"`) {
		t.Fatal("page has no url field")
	}

	v6 := get(r, http.MethodGet, "/app/", "[::1]:9", "")
	if !strings.Contains(v6.Body.String(), key) {
		t.Fatal("ipv6 loopback page omitted the admin key")
	}
}

func TestPageEscapesKey(t *testing.T) {
	const key = "secret-<tag>"
	r := mount(key, t.TempDir(), func() {}, func(string) error { return nil })
	res := get(r, http.MethodGet, "/app/", "127.0.0.1:1", "")
	body := res.Body.String()
	if strings.Contains(body, "secret-<tag>") {
		t.Fatal("raw key was written into HTML")
	}
	if !strings.Contains(body, `\u003c`) {
		t.Fatal("key was not JSON-escaped")
	}
}

func TestQuitAndFolderAreLocal(t *testing.T) {
	quit := false
	opened := ""
	dir := t.TempDir()
	r := mount("gui-test-key-0123456789abcdef", dir, func() { quit = true }, func(path string) error {
		opened = path
		return nil
	})

	denied := get(r, http.MethodPost, "/app/quit", "10.1.1.1:1234", "")
	if denied.Code != http.StatusForbidden || quit {
		t.Fatalf("remote quit status %d called %v", denied.Code, quit)
	}
	ok := get(r, http.MethodPost, "/app/quit", "127.0.0.1:9", "")
	if ok.Code != http.StatusNoContent || !quit {
		t.Fatalf("local quit status %d called %v", ok.Code, quit)
	}

	body := `{"path":"/etc/passwd"}`
	deniedFolder := get(r, http.MethodPost, "/app/folder", "10.1.1.1:1234", body)
	if deniedFolder.Code != http.StatusForbidden || opened != "" {
		t.Fatalf("remote folder status %d opened %q", deniedFolder.Code, opened)
	}
	okFolder := get(r, http.MethodPost, "/app/folder", "[::1]:9", body)
	if okFolder.Code != http.StatusNoContent || opened != dir {
		t.Fatalf("local folder status %d opened %q", okFolder.Code, opened)
	}
}

func TestAssets(t *testing.T) {
	r := mount("k", t.TempDir(), func() {}, func(string) error { return nil })
	for _, path := range []string{"/app/app.js", "/app/app.css"} {
		res := get(r, http.MethodGet, path, "10.1.1.1:1", "")
		if res.Code != http.StatusOK || res.Body.Len() == 0 {
			t.Fatalf("%s status %d len %d", path, res.Code, res.Body.Len())
		}
		if strings.Contains(res.Body.String(), "gui-test-key") {
			t.Fatalf("%s included a key", path)
		}
	}
	redir := get(r, http.MethodGet, "/app", "127.0.0.1:1", "")
	if redir.Code != http.StatusFound || redir.Header().Get("Location") != "/app/" {
		t.Fatalf("redirect %d %q", redir.Code, redir.Header().Get("Location"))
	}
}

func TestWindowURL(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:6543": "http://127.0.0.1:6543/app/",
		"[::]:6543":      "http://127.0.0.1:6543/app/",
		"0.0.0.0:6543":   "http://127.0.0.1:6543/app/",
		"192.168.1.9:80": "http://192.168.1.9:80/app/",
	}
	for addr, want := range cases {
		if got := WindowURL(addr); got != want {
			t.Fatalf("WindowURL(%s) = %s, want %s", addr, got, want)
		}
	}
}

func TestBrowserArgs(t *testing.T) {
	args := browserArgs("http://127.0.0.1:6543/app/", "/tmp/youtube gir")
	if len(args) < 2 {
		t.Fatal(args)
	}
	if args[0] != "--user-data-dir=/tmp/youtube gir" {
		t.Fatal(args[0])
	}
	if args[1] != "--app=http://127.0.0.1:6543/app/" {
		t.Fatal(args[1])
	}
}

func mount(key, dir string, quit func(), open func(string) error) *gin.Engine {
	r := gin.New()
	Mount(r, Deps{Key: key, DownloadDir: dir, Quit: quit, Open: open})
	return r
}

func get(r *gin.Engine, method, path, remote, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = remote
	r.ServeHTTP(w, req)
	return w
}
