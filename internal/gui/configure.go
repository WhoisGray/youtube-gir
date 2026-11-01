package gui

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/WhoisGray/youtube-gir/internal/config"
)

// Configure fills a data directory, admin key, and loopback port when the
// environment does not already set them, then loads the server config.
// ADMIN_KEY, DATA_DIR, and ADDR still win when they are set.
func Configure() (config.Config, error) {
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dir, err := userDataDir()
		if err != nil {
			return config.Config{}, err
		}
		dataDir = dir
		if err := os.Setenv("DATA_DIR", dataDir); err != nil {
			return config.Config{}, err
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return config.Config{}, err
	}
	if strings.TrimSpace(os.Getenv("ADMIN_KEY")) == "" {
		path := filepath.Join(dataDir, "admin.key")
		key, created, err := loadOrCreateKey(path)
		if err != nil {
			return config.Config{}, err
		}
		if err := os.Setenv("ADMIN_KEY", key); err != nil {
			return config.Config{}, err
		}
		if created {
			slog.Info("saved admin key", "path", path)
		}
	}
	if strings.TrimSpace(os.Getenv("ADDR")) == "" {
		addr, err := firstFree(dataDir, 6543)
		if err != nil {
			return config.Config{}, err
		}
		if err := os.Setenv("ADDR", addr); err != nil {
			return config.Config{}, err
		}
	}
	return config.Load()
}

func userDataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if app := strings.TrimSpace(os.Getenv("AppData")); app != "" {
			return filepath.Join(app, "youtube-gir"), nil
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "youtube-gir"), nil
	default:
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			return filepath.Join(xdg, "youtube-gir"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", "youtube-gir"), nil
	}
	return filepath.Join(home, ".local", "share", "youtube-gir"), nil
}

func loadOrCreateKey(path string) (string, bool, error) {
	key, err := readKey(path)
	if err == nil {
		return key, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, err
	}
	key = hex.EncodeToString(buf)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		key, err = readKey(path)
		return key, false, err
	}
	if err != nil {
		return "", false, err
	}
	_, werr := f.Write([]byte(key + "\n"))
	cerr := f.Close()
	if werr != nil {
		return "", false, werr
	}
	if cerr != nil {
		return "", false, cerr
	}
	return key, true, nil
}

func readKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(b))
	if len(key) < 16 {
		return "", fmt.Errorf("admin key file is too short")
	}
	return key, nil
}

func readPort(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 1 || n > 65535 {
		return 0
	}
	return n
}

// firstFree tries the saved port, then start and the next 39 ports.
func firstFree(dataDir string, start int) (string, error) {
	ports := make([]int, 0, 41)
	if p := readPort(filepath.Join(dataDir, "port")); p != 0 {
		ports = append(ports, p)
	}
	for p := start; p < start+40; p++ {
		ports = append(ports, p)
	}
	seen := map[int]bool{}
	var lastErr error
	for _, p := range ports {
		if seen[p] || p < 1 || p > 65535 {
			continue
		}
		seen[p] = true
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(p))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			lastErr = err
			continue
		}
		ln.Close()
		_ = os.WriteFile(filepath.Join(dataDir, "port"), []byte(strconv.Itoa(p)+"\n"), 0o644)
		return addr, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no free port")
	}
	return "", fmt.Errorf("no free local port: %w", lastErr)
}
