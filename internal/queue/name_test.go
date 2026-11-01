package queue

import "testing"

func TestFileName(t *testing.T) {
	if got := FileName(`a/b:c`, ".MP4"); got != "a-b-c.mp4" {
		t.Fatal(got)
	}
	if got := FileName("   ", ".m4a"); got != "download.m4a" {
		t.Fatal(got)
	}
	long := make([]rune, 200)
	for i := range long {
		long[i] = 'ن'
	}
	got := FileName(string(long), ".mp4")
	if len([]rune(got)) != 120+len(".mp4") {
		t.Fatalf("len %d %s", len([]rune(got)), got)
	}
}
