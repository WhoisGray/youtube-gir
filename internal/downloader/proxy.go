package downloader

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// NormalizeProxy checks a proxy URL for yt-dlp. An empty string clears it.
// Allowed schemes are the ones yt-dlp's --proxy flag accepts.
func NormalizeProxy(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 2048 || strings.ContainsAny(raw, " \t\r\n") {
		return "", errors.New("enter a proxy URL such as socks5://127.0.0.1:1080")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Opaque != "" {
		return "", errors.New("enter a proxy URL such as socks5://127.0.0.1:1080")
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h", "socks4", "socks4a", "http", "https":
	default:
		return "", errors.New("proxy scheme must be socks5, socks5h, socks4, socks4a, http, or https")
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("proxy URL cannot include a path or query")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("proxy URL needs a port from 1 to 65535")
	}
	u.Path = ""
	u.RawPath = ""
	u.Scheme = strings.ToLower(u.Scheme)
	return u.String(), nil
}
