package downloader

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in, page, id string
		ok           bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"https://youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"https://m.youtube.com/watch?v=abcdefghijk", "https://www.youtube.com/watch?v=abcdefghijk", "abcdefghijk", true},
		{"https://www.youtube.com/shorts/abc123XYZ_-", "https://www.youtube.com/shorts/abc123XYZ_-", "abc123XYZ_-", true},
		{"https://www.youtube.com/live/abcdefghijk", "https://www.youtube.com/watch?v=abcdefghijk", "abcdefghijk", true},
		{"https://music.youtube.com/watch?v=abcdefghijk", "https://www.youtube.com/watch?v=abcdefghijk", "abcdefghijk", true},
		{"https://example.com/watch?v=abcdefghijk", "", "", false},
		{"https://www.youtube.com.evil.com/watch?v=abcdefghijk", "", "", false},
		{"https://www.youtube.com/playlist?list=PL", "", "", false},
		{"file:///etc/passwd", "", "", false},
		{"https://www.youtube.com/watch?v=abc", "", "", false},
	}
	for _, tc := range cases {
		page, id, err := NormalizeURL(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s: expected error", tc.in)
		}
		if page != tc.page || id != tc.id {
			t.Fatalf("%s: got %s %s", tc.in, page, id)
		}
	}
}
