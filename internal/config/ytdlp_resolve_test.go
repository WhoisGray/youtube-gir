package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickYTDLPSkipsPython39(t *testing.T) {
	dir := t.TempDir()
	py := filepath.Join(dir, "python3.9")
	script := "#!/bin/sh\nif [ \"$1\" = \"-c\" ]; then echo '3 9'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(py, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(dir, "old")
	goodDir := filepath.Join(dir, "good")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(goodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(oldDir, "yt-dlp")
	good := filepath.Join(goodDir, "yt-dlp")
	if err := os.WriteFile(old, []byte("#!"+py+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := pickYTDLP([]string{old, good}); got != good {
		t.Fatalf("picked %s", got)
	}
	if got := pickYTDLP([]string{good, old}); got != good {
		t.Fatalf("picked %s", got)
	}
}

func TestPickYTDLPSkipsInstalledPython39(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(home, "Library/Python/3.9/bin/yt-dlp")
	good := filepath.Join(home, ".local/bin/yt-dlp")
	if _, err := os.Stat(old); err != nil {
		t.Skip(old)
	}
	if _, err := os.Stat(good); err != nil {
		t.Skip(good)
	}
	if got := pickYTDLP([]string{old, good}); got != good {
		t.Fatalf("picked %s", got)
	}
}

func TestFirstExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "deno")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := firstExecutable([]string{filepath.Join(dir, "missing"), bin}); got != bin {
		t.Fatalf("got %s", got)
	}
	if got := firstExecutable([]string{filepath.Join(dir, "missing")}); got != "" {
		t.Fatalf("got %s", got)
	}
}

func TestResolveYTDLPHonorsExplicitPath(t *testing.T) {
	if got := resolveYTDLP("/custom/yt-dlp"); got != "/custom/yt-dlp" {
		t.Fatalf("got %s", got)
	}
}
