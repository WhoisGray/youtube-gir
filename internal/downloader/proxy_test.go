package downloader

import "testing"

func TestNormalizeProxy(t *testing.T) {
	ok := []string{
		"socks5://127.0.0.1:1080",
		"SOCKS5://user:pass@10.0.0.2:1080",
		"socks5h://[::1]:1080",
		"http://proxy.example:8080",
		"https://proxy.example:443",
		"socks4://127.0.0.1:1080/",
	}
	for _, raw := range ok {
		got, err := NormalizeProxy(raw)
		if err != nil || got == "" {
			t.Fatalf("%s: %q %v", raw, got, err)
		}
	}
	cleared, err := NormalizeProxy("  ")
	if err != nil || cleared != "" {
		t.Fatalf("clear: %q %v", cleared, err)
	}
	bad := []string{
		"socks5://127.0.0.1",
		"ftp://127.0.0.1:1080",
		"socks5://127.0.0.1:1080/path",
		"socks5://127.0.0.1:1080?x=1",
		"file:///tmp/proxy",
		"127.0.0.1:1080",
		"socks5://127.0.0.1:99999",
	}
	for _, raw := range bad {
		if _, err := NormalizeProxy(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
