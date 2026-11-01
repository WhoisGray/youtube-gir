package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/WhoisGray/youtube-gir/internal/config"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestRunServesAndStops(t *testing.T) {
	t.Setenv("ADMIN_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("ADDR", "127.0.0.1:0")
	t.Setenv("JS_RUNTIME", "none")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, Options{
			Ready: func(addr string) { ready <- addr },
			Mount: func(engine *gin.Engine) {
				engine.GET("/app/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
			},
		})
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(8 * time.Second):
		t.Fatal("server did not become ready")
	}

	var health *http.Response
	deadline := time.Now().Add(3 * time.Second)
	for {
		health, err = http.Get("http://" + addr + "/health")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	body, _ := io.ReadAll(health.Body)
	health.Body.Close()
	if health.StatusCode != http.StatusOK && health.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("health %d %s", health.StatusCode, body)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["version"] != config.Version {
		t.Fatalf("version %#v", doc["version"])
	}

	root, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	rootBody, _ := io.ReadAll(root.Body)
	root.Body.Close()
	if err := json.Unmarshal(rootBody, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "youtube-gir" {
		t.Fatalf("root %#v", doc["name"])
	}

	ping, err := http.Get("http://" + addr + "/app/ping")
	if err != nil {
		t.Fatal(err)
	}
	ping.Body.Close()
	if ping.StatusCode != http.StatusOK {
		t.Fatalf("ping %d", ping.StatusCode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("server did not stop")
	}
}
