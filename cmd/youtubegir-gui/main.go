package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/WhoisGray/youtube-gir/internal/app"
	"github.com/WhoisGray/youtube-gir/internal/gui"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dataDir, err := gui.DataDir()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	gui.LogToFile(dataDir)

	release, already, err := gui.ClaimRunning(dataDir)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if already {
		focusExisting(dataDir)
		return
	}
	defer release()

	cfg, err := gui.Configure()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		cancel()
	}()

	var win *gui.Window
	err = app.Run(ctx, cfg, app.Options{
		Mount: func(engine *gin.Engine) {
			gui.Mount(engine, gui.Deps{
				Key:         cfg.AdminKey,
				DownloadDir: cfg.DownloadDir,
				Quit:        cancel,
			})
		},
		Ready: func(addr string) {
			url := gui.WindowURL(addr)
			slog.Info("window", "url", url)
			if os.Getenv("YG_GUI_NO_BROWSER") == "1" {
				return
			}
			launched, launchErr := gui.Launch(url)
			if launchErr != nil {
				slog.Error("browser", "err", launchErr)
				slog.Info("server stays up until you stop it", "url", url)
				return
			}
			win = launched
			go func() {
				started := time.Now()
				waitErr := launched.Wait()
				launched.RemoveProfile()
				// Chrome sometimes returns at once. That is not the window closing.
				if time.Since(started) < time.Second {
					slog.Warn("browser exited immediately; the server stays up", "err", waitErr, "url", url)
					return
				}
				cancel()
			}()
		},
	})
	if win != nil {
		win.Kill()
	}
	if err != nil {
		slog.Error("gui", "err", err)
		os.Exit(1)
	}
}

func focusExisting(dataDir string) {
	if os.Getenv("YG_GUI_NO_BROWSER") == "1" {
		slog.Error("already running")
		os.Exit(1)
	}
	port := gui.SavedPort(dataDir)
	if port == 0 {
		slog.Error("already running")
		os.Exit(1)
	}
	url := gui.WindowURL(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	slog.Info("already running", "url", url)
	if _, err := gui.Launch(url); err != nil {
		slog.Error("browser", "err", err)
		os.Exit(1)
	}
}
