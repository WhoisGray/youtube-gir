package downloader

import "testing"

func TestNormalizeQuality(t *testing.T) {
	q, err := NormalizeQuality("video", "")
	if err != nil || q != "best" {
		t.Fatalf("best: %s %v", q, err)
	}
	q, err = NormalizeQuality("video", "1080")
	if err != nil || q != "1080" {
		t.Fatalf("1080: %s %v", q, err)
	}
	q, err = NormalizeQuality("video", "Car")
	if err != nil || q != "car" {
		t.Fatalf("car: %s %v", q, err)
	}
	q, err = NormalizeQuality("video", "137")
	if err != nil || q != "137" {
		t.Fatalf("format id 137 should stay a format id: %s %v", q, err)
	}
	q, err = NormalizeQuality("audio", "1080")
	if err != nil || q != "best" {
		t.Fatalf("audio: %s %v", q, err)
	}
	if _, err := NormalizeQuality("video", "--exec"); err == nil {
		t.Fatal("flag-like quality must be rejected")
	}
	if _, err := NormalizeQuality("video", "best;rm"); err == nil {
		t.Fatal("punctuation must be rejected")
	}
	if _, err := NormalizeQuality("nope", "best"); err == nil {
		t.Fatal("bad kind")
	}
}

func TestFormatArgs(t *testing.T) {
	args, err := FormatArgs("video", "720")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, a := range args {
		joined += a + " "
	}
	if args[0] != "-f" || args[1] != "bv*[height<=720]+ba/b[height<=720]" {
		t.Fatalf("unexpected args: %v", args)
	}
	args, err = FormatArgs("video", "car")
	if err != nil {
		t.Fatal(err)
	}
	if args[0] != "-f" || args[1] != carFormat || args[2] != "--merge-output-format" || args[3] != "mp4" || args[4] != "--postprocessor-args" || args[5] != carMerge {
		t.Fatalf("car args: %#v", args)
	}
	args, err = FormatArgs("audio", "")
	if err != nil || args[2] != "-x" {
		t.Fatalf("audio args: %v %v", args, err)
	}
}

func TestProbeQualities(t *testing.T) {
	p := probeFromDump(rawDump{
		ID:    "abcdefghijk",
		Title: "Song",
		Formats: []rawFormat{
			{FormatID: "140", Ext: "m4a", ACodec: "mp4a", VCodec: "none", Filesize: 1000, TBR: 128},
			{FormatID: "137", Ext: "mp4", VCodec: "avc1", ACodec: "none", Height: 1080, Filesize: 5000, TBR: 4000, FPS: 30},
			{FormatID: "136", Ext: "mp4", VCodec: "avc1", ACodec: "none", Height: 720, Filesize: 2000, TBR: 2000, FPS: 30},
			{FormatID: "sb0", Ext: "mhtml", VCodec: "none", ACodec: "none"},
		},
	})
	if p.Title != "Song" || len(p.Formats) != 3 {
		t.Fatalf("formats: %+v", p.Formats)
	}
	if len(p.Qualities) != 2 {
		t.Fatalf("qualities: %+v", p.Qualities)
	}
	if p.Qualities[0].ID != "1080" || p.Qualities[0].Filesize != 6000 {
		t.Fatalf("1080 size should include audio: %+v", p.Qualities[0])
	}
	if p.Qualities[1].ID != "720" || p.Qualities[1].Filesize != 3000 {
		t.Fatalf("720: %+v", p.Qualities[1])
	}
}
