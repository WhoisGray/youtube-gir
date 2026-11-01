// Package app starts the youtube-gir server used by the headless binary and the desktop app.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/WhoisGray/youtube-gir/internal/config"
	"github.com/WhoisGray/youtube-gir/internal/downloader"
	"github.com/WhoisGray/youtube-gir/internal/httpapi"
	"github.com/WhoisGray/youtube-gir/internal/queue"
	"github.com/WhoisGray/youtube-gir/internal/store"
)

// Options adds routes and a callback once the port is open.
type Options struct {
	Mount func(*gin.Engine)
	Ready func(addr string)
}

// Run serves until ctx is cancelled or the listener fails.
func Run(ctx context.Context, cfg config.Config, opts Options) error {
	if len(cfg.AdminKey) < 16 {
		slog.Warn("ADMIN_KEY is short; use a long random string")
	}
	if cfg.CookiesFile != "" {
		if _, err := os.Stat(cfg.CookiesFile); err != nil {
			slog.Warn("COOKIES_FILE is not readable; ignoring it", "path", cfg.CookiesFile, "err", err)
			cfg.CookiesFile = ""
		}
	}
	for _, dir := range []string{cfg.DataDir, cfg.DownloadDir, cfg.TempDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.EnsureAdmin(context.Background(), cfg.AdminKey); err != nil {
		return err
	}

	runner := downloader.New(cfg.YTDLPPath, cfg.FFmpegPath, cfg.JSRuntime, cfg.CookiesFile)
	proxy, err := seedDownloadProxy(st)
	if err != nil {
		return err
	}
	if proxy != "" {
		runner.SetProxy(proxy)
		slog.Info("download proxy enabled")
	}
	if !runner.Available() {
		slog.Warn("yt-dlp was not found; downloads will fail until it is installed", "path", cfg.YTDLPPath)
	} else if old, ok := config.PythonTooOld(cfg.YTDLPPath); ok && old {
		slog.Warn("yt-dlp is running on Python older than 3.10; YouTube downloads need a current binary", "path", cfg.YTDLPPath)
	} else {
		slog.Info("yt-dlp", "path", cfg.YTDLPPath)
	}

	mgr := queue.New(st, runner, cfg)
	engine := httpapi.Router(httpapi.New(st, mgr, runner, cfg))
	if opts.Mount != nil {
		opts.Mount(engine)
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	addr := ln.Addr().String()
	slog.Info("listening", "addr", addr, "version", config.Version)
	if opts.Ready != nil {
		opts.Ready(addr)
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	mgr.Shutdown()
	shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shut); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func seedDownloadProxy(st *store.Store) (string, error) {
	ctx := context.Background()
	value, ok, err := st.GetSetting(ctx, store.SettingDownloadProxy)
	if err != nil {
		return "", err
	}
	if !ok {
		env := strings.TrimSpace(os.Getenv("DOWNLOAD_PROXY"))
		if env == "" {
			return "", nil
		}
		norm, err := downloader.NormalizeProxy(env)
		if err != nil {
			slog.Warn("DOWNLOAD_PROXY ignored", "err", err)
			return "", nil
		}
		if err := st.SetSetting(ctx, store.SettingDownloadProxy, norm); err != nil {
			return "", err
		}
		return norm, nil
	}
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	norm, err := downloader.NormalizeProxy(value)
	if err != nil {
		slog.Warn("stored download proxy ignored", "err", err)
		return "", nil
	}
	return norm, nil
}
