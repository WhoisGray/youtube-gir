package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/WhoisGray/youtube-gir/internal/config"
	"github.com/WhoisGray/youtube-gir/internal/downloader"
	"github.com/WhoisGray/youtube-gir/internal/httpapi"
	"github.com/WhoisGray/youtube-gir/internal/queue"
	"github.com/WhoisGray/youtube-gir/internal/store"
)

// seedDownloadProxy loads the saved proxy. DOWNLOAD_PROXY is written only
// when the admin has never saved one, so clearing it in the app stays cleared.
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

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
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
			slog.Error("create directory", "path", dir, "err", err)
			os.Exit(1)
		}
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := st.EnsureAdmin(context.Background(), cfg.AdminKey); err != nil {
		slog.Error("ensure admin", "err", err)
		os.Exit(1)
	}

	runner := downloader.New(cfg.YTDLPPath, cfg.FFmpegPath, cfg.JSRuntime, cfg.CookiesFile)
	proxy, err := seedDownloadProxy(st)
	if err != nil {
		slog.Error("download proxy", "err", err)
		os.Exit(1)
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

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "version", config.Version)
		errCh <- srv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	case s := <-sig:
		slog.Info("shutting down", "signal", s.String())
	}

	mgr.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown", "err", err)
		os.Exit(1)
	}
}
