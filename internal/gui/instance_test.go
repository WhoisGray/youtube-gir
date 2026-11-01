//go:build unix

package gui

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestClaimRunningBlocksOtherProcess(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "YG_LOCK_HELPER=1", "YG_LOCK_DIR="+dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = stdin.Close()
			_ = cmd.Wait()
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	held := false
	for time.Now().Before(deadline) {
		release, already, err := ClaimRunning(dir)
		if err != nil {
			t.Fatal(err)
		}
		if already {
			held = true
			break
		}
		release()
		time.Sleep(20 * time.Millisecond)
	}
	if !held {
		t.Fatalf("other process did not hold the lock: %s", stderr.String())
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("%v: %s", err, stderr.String())
	}
	finished = true

	deadline = time.Now().Add(2 * time.Second)
	for {
		release, already, err := ClaimRunning(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !already {
			release()
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("lock was still held after the other process exited")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestJoinNewPath(t *testing.T) {
	dir := t.TempDir()
	got := joinNewPath("/usr/bin:/bin", []string{dir, dir, "/no/such/youtubegir-path"})
	sep := string(os.PathListSeparator)
	want := dir + sep + "/usr/bin:/bin"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if again := joinNewPath(got, []string{dir}); again != got {
		t.Fatalf("duplicated path %q", again)
	}
	if empty := joinNewPath("", []string{"/no/such/youtubegir-path"}); empty != "" {
		t.Fatalf("missing dir was added: %q", empty)
	}
}
