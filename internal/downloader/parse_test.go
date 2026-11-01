package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLine(t *testing.T) {
	ev, ok := ParseLine("[download]  12.5% of  10.00MiB at  1.25MiB/s ETA 00:07")
	if !ok || ev.Kind != "progress" || ev.Percent != 12.5 || ev.TotalBytes != 10*1024*1024 || ev.SpeedBPS != int64(1.25*1024*1024) || ev.ETA != "00:07" {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine("[download] 100% of 10.00MiB in 00:03")
	if !ok || ev.Percent != 100 || ev.TotalBytes != 10*1024*1024 || ev.HasSpeed {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine("[download]   0.0% of ~  50.20MiB at  Unknown B/s ETA Unknown")
	if !ok || ev.Percent != 0 || ev.TotalBytes != 52638515 || ev.HasSpeed || ev.ETA != "" {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine("[download]  33.3% of ~  1.07MiB at  512.00KiB/s ETA 00:02 (frag 2/6)")
	if !ok || ev.ETA != "00:02" || ev.SpeedBPS != 512*1024 {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine(`[download] Destination: /tmp/foo.mp4`)
	if !ok || ev.Path != "/tmp/foo.mp4" {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine(`[Merger] Merging formats into "/tmp/foo.mp4"`)
	if !ok || ev.Path != "/tmp/foo.mp4" {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine(`[ExtractAudio] Destination: /tmp/foo.m4a`)
	if !ok || ev.Path != "/tmp/foo.m4a" {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine(`[download] /tmp/foo.mp4 has already been downloaded`)
	if !ok || ev.Path != "/tmp/foo.mp4" {
		t.Fatalf("%+v", ev)
	}
	ev, ok = ParseLine("ERROR: Private video")
	if !ok || ev.Kind != "error" || ev.Text != "Private video" {
		t.Fatalf("%+v", ev)
	}
	if _, ok := ParseLine("WARNING: something"); ok {
		t.Fatal("warnings are ignored")
	}
}

func TestFindOutputSkipsPartFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "clip.mp4.part"), []byte("partial-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindOutput(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "clip.mp4" {
		t.Fatal(got)
	}
}
