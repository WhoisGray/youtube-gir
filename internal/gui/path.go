package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// extendToolPath adds the usual install locations in front of PATH.
// A double-clicked Mac app only sees /usr/bin and /bin.
func extendToolPath() {
	if runtime.GOOS == "windows" {
		return
	}
	_ = os.Setenv("PATH", joinNewPath(os.Getenv("PATH"), toolPathDirs()))
}

func toolPathDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	home, err := os.UserHomeDir()
	if err != nil {
		return dirs
	}
	return append([]string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".deno", "bin"),
	}, dirs...)
}

func joinNewPath(current string, dirs []string) string {
	sep := string(os.PathListSeparator)
	have := map[string]bool{}
	for _, dir := range filepath.SplitList(current) {
		have[dir] = true
	}
	var add []string
	for _, dir := range dirs {
		if dir == "" || have[dir] {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		have[dir] = true
		add = append(add, dir)
	}
	if len(add) == 0 {
		return current
	}
	if current == "" {
		return strings.Join(add, sep)
	}
	return strings.Join(add, sep) + sep + current
}
