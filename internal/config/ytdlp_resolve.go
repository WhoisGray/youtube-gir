package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// resolveYTDLP picks a yt-dlp binary. An explicit YTDLP_PATH is used as given.
// Otherwise the first executable on PATH that is not a Python 3.9 launcher
// wins. YouTube rejects that old interpreter with "the page needs to be
// reloaded", so a current binary later on PATH is preferred.
func resolveYTDLP(explicit string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit
	}
	candidates := ytdlpCandidates()
	if len(candidates) == 0 {
		return "yt-dlp"
	}
	return pickYTDLP(candidates)
}

func ytdlpCandidates() []string {
	var out []string
	seen := map[string]bool{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		add(filepath.Join(dir, "yt-dlp"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".local", "bin", "yt-dlp"))
	}
	add("/opt/homebrew/bin/yt-dlp")
	add("/usr/local/bin/yt-dlp")
	return out
}

func pickYTDLP(candidates []string) string {
	fallback := ""
	for _, path := range candidates {
		if fallback == "" {
			fallback = path
		}
		if old, ok := PythonTooOld(path); ok && old {
			continue
		}
		return path
	}
	return fallback
}

// PythonTooOld reports whether path is a Python launcher older than 3.10.
func PythonTooOld(path string) (bool, bool) {
	major, minor, ok := pythonLauncherVersion(path)
	if !ok {
		return false, false
	}
	return major < 3 || (major == 3 && minor < 10), true
}

func pythonLauncherVersion(path string) (major, minor int, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	line, _, _ := strings.Cut(string(buf[:n]), "\n")
	if !strings.HasPrefix(line, "#!") || !strings.Contains(strings.ToLower(line), "python") {
		return 0, 0, false
	}
	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	if len(fields) == 0 {
		return 0, 0, false
	}
	interp := fields[0]
	if interp == "/usr/bin/env" {
		if len(fields) < 2 {
			return 0, 0, false
		}
		interp = fields[1]
		if resolved, err := exec.LookPath(interp); err == nil {
			interp = resolved
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, interp, "-c", "import sys; print(sys.version_info[0], sys.version_info[1])").Output()
	if err != nil {
		return 0, 0, false
	}
	parts := strings.Fields(string(out))
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}
