package downloader

import (
	"errors"
	"net/url"
	"strings"
)

// ErrInvalidURL means the value is not a single YouTube video URL.
var ErrInvalidURL = errors.New("only YouTube video URLs are supported")

// NormalizeURL checks the host and returns a canonical watch or shorts URL
// plus the video id. Playlist and tracking parameters are dropped so the
// same video queues once.
func NormalizeURL(raw string) (page string, videoID string, err error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", "", ErrInvalidURL
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", "", ErrInvalidURL
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "youtu.be":
		id := firstSegment(u.Path)
		if !validVideoID(id) {
			return "", "", ErrInvalidURL
		}
		return "https://www.youtube.com/watch?v=" + id, id, nil
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com", "www.youtube-nocookie.com":
		if strings.HasPrefix(u.Path, "/shorts/") {
			id := firstSegment(strings.TrimPrefix(u.Path, "/shorts/"))
			if !validVideoID(id) {
				return "", "", ErrInvalidURL
			}
			return "https://www.youtube.com/shorts/" + id, id, nil
		}
		if strings.HasPrefix(u.Path, "/live/") {
			id := firstSegment(strings.TrimPrefix(u.Path, "/live/"))
			if !validVideoID(id) {
				return "", "", ErrInvalidURL
			}
			return "https://www.youtube.com/watch?v=" + id, id, nil
		}
		if u.Path == "/watch" || strings.HasPrefix(u.Path, "/watch/") {
			id := u.Query().Get("v")
			if !validVideoID(id) {
				return "", "", ErrInvalidURL
			}
			return "https://www.youtube.com/watch?v=" + id, id, nil
		}
		return "", "", ErrInvalidURL
	default:
		return "", "", ErrInvalidURL
	}
}

func firstSegment(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	return strings.Split(path, "/")[0]
}

func validVideoID(id string) bool {
	if len(id) < 6 || len(id) > 32 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
