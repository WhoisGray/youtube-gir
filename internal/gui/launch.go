package gui

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Window is a browser window opened against the local app page.
type Window struct {
	cmd     *exec.Cmd
	profile string
}

// Launch starts Chrome, Edge, Chromium, or Brave in app mode.
func Launch(url string) (*Window, error) {
	bin, err := findBrowser()
	if err != nil {
		return nil, err
	}
	profile, err := os.MkdirTemp("", "youtube-gir-gui-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, browserArgs(url, profile)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(profile)
		return nil, err
	}
	return &Window{cmd: cmd, profile: profile}, nil
}

// Wait blocks until the browser process exits.
func (w *Window) Wait() error {
	if w == nil || w.cmd == nil {
		return nil
	}
	return w.cmd.Wait()
}

// Kill stops the browser process if it is still running.
func (w *Window) Kill() {
	if w == nil || w.cmd == nil || w.cmd.Process == nil {
		return
	}
	_ = w.cmd.Process.Kill()
}

// RemoveProfile deletes the temporary browser profile.
func (w *Window) RemoveProfile() {
	if w == nil || w.profile == "" {
		return
	}
	_ = os.RemoveAll(w.profile)
	w.profile = ""
}

func browserArgs(url, profile string) []string {
	return []string{
		"--user-data-dir=" + profile,
		"--app=" + url,
		"--window-size=1080,760",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--disable-extensions",
	}
}

// WindowURL is the page a local browser should open for a listen address.
func WindowURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr + "/app/"
	}
	switch host {
	case "", "::", "0.0.0.0", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/app/"
}

func findBrowser() (string, error) {
	for _, path := range browserCandidates() {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, nil
		}
	}
	for _, name := range []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"microsoft-edge",
		"msedge",
		"brave-browser",
		"brave",
	} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("install Chrome, Edge, Chromium, or Brave to open the window")
}

func browserCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	case "windows":
		roots := []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("LocalAppData"),
		}
		rels := []string{
			filepath.Join("Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join("BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		}
		var out []string
		for _, root := range roots {
			if root == "" {
				continue
			}
			for _, rel := range rels {
				out = append(out, filepath.Join(root, rel))
			}
		}
		return out
	default:
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
			"/snap/bin/chromium",
		}
	}
}
