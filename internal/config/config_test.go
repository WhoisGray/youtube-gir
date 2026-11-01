package config

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ADMIN_KEY", "abcdefghijklmnopqrstuvwxyz")
	t.Setenv("DATA_DIR", dir)
	t.Setenv("ADDR", "9090")
	t.Setenv("MAX_CONCURRENT", "4")
	t.Setenv("DB_PATH", "")
	t.Setenv("DOWNLOAD_DIR", "")
	t.Setenv("TEMP_DIR", "")
	t.Setenv("YTDLP_PATH", "")
	t.Setenv("FFMPEG_PATH", "")
	t.Setenv("COOKIES_FILE", "")
	t.Setenv("JS_RUNTIME", "none")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" || cfg.MaxConcurrent != 4 || cfg.JSRuntime != "" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.DBPath != filepath.Join(dir, "db", "youtubegir.sqlite") {
		t.Fatal(cfg.DBPath)
	}
	if cfg.DownloadDir != filepath.Join(dir, "downloads") || cfg.TempDir != filepath.Join(dir, "tmp") {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadRequiresAdminKey(t *testing.T) {
	t.Setenv("ADMIN_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}
