//go:build unix

package downloader

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecTimeoutKillsProcessGroup(t *testing.T) {
	y := New("/bin/sh", "", "", "")
	start := time.Now()
	_, err := y.exec(context.Background(), 400*time.Millisecond, false, "-c", "sleep 30")
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("timeout took %s; the process group was not killed", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestExecReturnsStdout(t *testing.T) {
	y := New("/bin/sh", "", "", "")
	out, err := y.exec(context.Background(), 5*time.Second, false, "-c", "printf ok")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ok" {
		t.Fatalf("stdout = %q", out)
	}
}
