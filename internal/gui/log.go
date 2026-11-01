package gui

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LogToFile keeps a copy of the log when the process has no terminal.
// A double-clicked app otherwise discards its output.
func LogToFile(dataDir string) {
	info, err := os.Stdout.Stat()
	if err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return
	}
	path := filepath.Join(dataDir, "gui.log")
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		_ = os.Remove(path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, f), &slog.HandlerOptions{Level: slog.LevelInfo})))
}
