// Package config loads runtime settings from the environment.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Version is the server version. A release build replaces it with -ldflags.
var Version = "1.0.0"

// Config is the process configuration. Paths are absolute.
type Config struct {
	Addr          string
	AdminKey      string
	DataDir       string
	DBPath        string
	DownloadDir   string
	TempDir       string
	MaxConcurrent int
	YTDLPPath     string
	FFmpegPath    string
	JSRuntime     string
	CookiesFile   string
}

// Load reads environment variables and fills defaults.
// ADMIN_KEY is required. JS_RUNTIME=none disables the JavaScript runtime.
// An empty JS_RUNTIME uses deno when it is on PATH, otherwise node.
func Load() (Config, error) {
	dataDir := abs(env("DATA_DIR", "data"))
	cfg := Config{
		Addr:          env("ADDR", ":6543"),
		AdminKey:      strings.TrimSpace(os.Getenv("ADMIN_KEY")),
		DataDir:       dataDir,
		DBPath:        abs(env("DB_PATH", filepath.Join(dataDir, "db", "youtubegir.sqlite"))),
		DownloadDir:   abs(env("DOWNLOAD_DIR", filepath.Join(dataDir, "downloads"))),
		TempDir:       abs(env("TEMP_DIR", filepath.Join(dataDir, "tmp"))),
		MaxConcurrent: 2,
		YTDLPPath:     resolveYTDLP(os.Getenv("YTDLP_PATH")),
		FFmpegPath:    strings.TrimSpace(os.Getenv("FFMPEG_PATH")),
		CookiesFile:   strings.TrimSpace(os.Getenv("COOKIES_FILE")),
	}
	if cfg.AdminKey == "" {
		return Config{}, fmt.Errorf("ADMIN_KEY is required")
	}
	if !strings.Contains(cfg.Addr, ":") {
		cfg.Addr = ":" + cfg.Addr
	}
	if raw := strings.TrimSpace(os.Getenv("MAX_CONCURRENT")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("MAX_CONCURRENT must be a positive integer")
		}
		if n > 32 {
			n = 32
		}
		cfg.MaxConcurrent = n
	}
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = findFFmpeg()
	}
	cfg.JSRuntime = resolveJSRuntime()
	return cfg, nil
}

func resolveJSRuntime() string {
	switch strings.TrimSpace(os.Getenv("JS_RUNTIME")) {
	case "none":
		return ""
	case "":
		if p := findDeno(); p != "" {
			return "deno:" + p
		}
		if p := findNode(); p != "" {
			return "node:" + p
		}
		return ""
	default:
		return strings.TrimSpace(os.Getenv("JS_RUNTIME"))
	}
}

// findDeno looks on PATH and in the usual install locations. A login shell
// can omit Homebrew's bin, which is where the Mac installer puts deno.
func findDeno() string {
	if p, err := exec.LookPath("deno"); err == nil {
		return p
	}
	candidates := []string{"/opt/homebrew/bin/deno", "/usr/local/bin/deno"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".deno", "bin", "deno"),
			filepath.Join(home, ".local", "bin", "deno"),
		)
	}
	return firstExecutable(candidates)
}

func findNode() string {
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	candidates := []string{"/opt/homebrew/bin/node", "/usr/local/bin/node"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "node"))
	}
	return firstExecutable(candidates)
}

func findFFmpeg() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return firstExecutable([]string{"/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg"})
}

func firstExecutable(paths []string) string {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return path
		}
	}
	return ""
}

func env(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func abs(path string) string {
	a, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return a
}
