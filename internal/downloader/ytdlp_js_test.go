package downloader

import (
	"errors"
	"strings"
	"testing"
)

func TestBaseDropsUnsupportedJSRuntimes(t *testing.T) {
	y := New("yt-dlp", "", "deno:/usr/local/bin/deno", "")
	joined := strings.Join(y.base(), " ")
	if !strings.Contains(joined, "--js-runtimes deno:/usr/local/bin/deno") {
		t.Fatalf("args = %s", joined)
	}
	err := errors.New("yt-dlp: error: no such option: --js-runtimes")
	if !y.dropUnsupportedJS(err) {
		t.Fatal("expected a retry")
	}
	if strings.Contains(strings.Join(y.base(), " "), "--js-runtimes") {
		t.Fatal("flag still present")
	}
	if y.dropUnsupportedJS(err) {
		t.Fatal("second rejection should not retry again")
	}
}

func TestBaseProxy(t *testing.T) {
	y := New("yt-dlp", "", "", "")
	if strings.Contains(strings.Join(y.base(), " "), "--proxy") {
		t.Fatal("proxy flag without a setting")
	}
	y.SetProxy("socks5://127.0.0.1:1080")
	args := y.base()
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--proxy" && args[i+1] == "socks5://127.0.0.1:1080" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args = %v", args)
	}
	y.SetProxy("")
	if strings.Contains(strings.Join(y.base(), " "), "--proxy") {
		t.Fatal("cleared proxy still present")
	}
}

func TestCleanYTDLPOutputDropsPythonWarning(t *testing.T) {
	in := "" +
		"/Users/paliz/Library/Python/3.9/lib/python/site-packages/urllib3/__init__.py:35: NotOpenSSLWarning: urllib3 v2 only supports OpenSSL 1.1.1+\n" +
		"  warnings.warn(\n" +
		"Usage: yt-dlp [OPTIONS] URL [URL...]\n" +
		"yt-dlp: error: no such option: --js-runtimes\n"
	got := cleanYTDLPOutput(in)
	want := "yt-dlp: error: no such option: --js-runtimes"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
