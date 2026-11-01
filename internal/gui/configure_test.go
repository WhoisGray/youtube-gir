package gui

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigureHonorsEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ADMIN_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("DATA_DIR", dir)
	t.Setenv("ADDR", "127.0.0.1:9")
	cfg, err := Configure()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminKey != "0123456789abcdef0123456789abcdef" {
		t.Fatal("admin key was replaced")
	}
	if cfg.Addr != "127.0.0.1:9" {
		t.Fatalf("addr %s", cfg.Addr)
	}
	if _, err := os.Stat(filepath.Join(dir, "admin.key")); !os.IsNotExist(err) {
		t.Fatal("wrote admin.key even though ADMIN_KEY was set")
	}
}

func TestConfigureCreatesKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ADMIN_KEY", "")
	t.Setenv("DATA_DIR", dir)
	t.Setenv("ADDR", "127.0.0.1:9")
	cfg, err := Configure()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AdminKey) != 64 {
		t.Fatalf("key length %d", len(cfg.AdminKey))
	}
	info, err := os.Stat(filepath.Join(dir, "admin.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("admin.key mode %o", info.Mode().Perm())
	}
	again, err := Configure()
	if err != nil {
		t.Fatal(err)
	}
	if again.AdminKey != cfg.AdminKey {
		t.Fatal("admin key changed on the second start")
	}
}

func TestConfigureRejectsShortKeyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "admin.key"), []byte("short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_KEY", "")
	t.Setenv("DATA_DIR", dir)
	t.Setenv("ADDR", "127.0.0.1:9")
	if _, err := Configure(); err == nil {
		t.Fatal("expected a short key file to fail")
	}
}

func TestFirstFreeSkipsBusyPort(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "port"), []byte(port+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var start int
	for _, c := range port {
		start = start*10 + int(c-'0')
	}
	addr, err := firstFree(dir, start)
	if err != nil {
		t.Fatal(err)
	}
	if addr == "127.0.0.1:"+port {
		t.Fatal("chose the busy port")
	}
	saved, err := os.ReadFile(filepath.Join(dir, "port"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(saved)) == port {
		t.Fatal("saved port was left on the busy port")
	}
}

func TestUserDataDir(t *testing.T) {
	dir, err := userDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, "youtube-gir") {
		t.Fatal(dir)
	}
	if runtime.GOOS == "darwin" && !strings.Contains(dir, filepath.Join("Library", "Application Support", "youtube-gir")) {
		t.Fatal(dir)
	}
}
