// Package httpapi is the REST API used by the extension and curl.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/WhoisGray/youtube-gir/internal/config"
	"github.com/WhoisGray/youtube-gir/internal/downloader"
	"github.com/WhoisGray/youtube-gir/internal/queue"
	"github.com/WhoisGray/youtube-gir/internal/store"
)

const userKey = "authUser"

var (
	downloadIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	fileTokenPattern  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	usernamePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{1,31}$`)
)

// Handler serves the API.
type Handler struct {
	store  *store.Store
	queue  *queue.Manager
	runner downloader.Runner
	cfg    config.Config
}

func New(st *store.Store, q *queue.Manager, runner downloader.Runner, cfg config.Config) *Handler {
	return &Handler{store: st, queue: q, runner: runner, cfg: cfg}
}

// Router builds the HTTP handler.
func Router(h *Handler) *gin.Engine {
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(gin.Recovery(), requestLog(), cors(), limitBody(1<<20))
	r.GET("/", h.root)
	r.GET("/health", h.health)
	r.GET("/api/v1/files/:token", h.downloadFile)

	api := r.Group("/api/v1", h.authenticate)
	api.GET("/me", h.me)
	api.GET("/stats", h.stats)
	api.POST("/formats", h.formats)
	api.GET("/downloads", h.listDownloads)
	api.POST("/downloads", h.createDownload)
	api.GET("/downloads/:id", h.getDownload)
	api.POST("/downloads/:id/pause", h.pause)
	api.POST("/downloads/:id/resume", h.resume)
	api.POST("/downloads/:id/cancel", h.cancel)
	api.DELETE("/downloads/:id", h.deleteDownload)

	admin := api.Group("", h.requireAdmin)
	admin.POST("/users", h.createUser)
	admin.GET("/users", h.listUsers)
	admin.DELETE("/users/:id", h.deleteUser)
	admin.GET("/system/ytdlp", h.ytdlpVersion)
	admin.POST("/system/ytdlp/update", h.updateYTDLP)
	admin.GET("/system/proxy", h.getProxy)
	admin.PUT("/system/proxy", h.setProxy)
	return r
}

func (h *Handler) root(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":    "youtube-gir",
		"version": config.Version,
		"health":  "/health",
		"api":     "/api/v1",
	})
}

func (h *Handler) health(c *gin.Context) {
	ytdlp := h.runner.Available()
	ffmpeg := false
	if h.cfg.FFmpegPath != "" {
		if st, err := os.Stat(h.cfg.FFmpegPath); err == nil && !st.IsDir() {
			ffmpeg = true
		}
	}
	if !ffmpeg {
		_, err := exec.LookPath("ffmpeg")
		ffmpeg = err == nil
	}
	status := "ok"
	code := http.StatusOK
	if !ytdlp {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{
		"status":  status,
		"version": config.Version,
		"ytdlp":   ytdlp,
		"ffmpeg":  ffmpeg,
	})
}

func (h *Handler) authenticate(c *gin.Context) {
	token := bearer(c)
	if token == "" {
		fail(c, http.StatusUnauthorized, "unauthorized", "missing API key")
		c.Abort()
		return
	}
	u, err := h.store.UserByToken(c.Request.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		fail(c, http.StatusUnauthorized, "unauthorized", "invalid API key")
		c.Abort()
		return
	}
	if err != nil {
		slog.Error("auth lookup", "err", err)
		fail(c, http.StatusInternalServerError, "internal", "could not check API key")
		c.Abort()
		return
	}
	c.Set(userKey, u)
	c.Next()
}

func (h *Handler) requireAdmin(c *gin.Context) {
	if userFrom(c).Role != store.RoleAdmin {
		fail(c, http.StatusForbidden, "forbidden", "admin key required")
		c.Abort()
		return
	}
	c.Next()
}

func (h *Handler) me(c *gin.Context) {
	u := userFrom(c)
	stats, err := h.store.Stats(c.Request.Context(), u.ID, false)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load stats")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":       u.ID,
		"username": u.Username,
		"role":     u.Role,
		"stats":    stats,
	})
}

func (h *Handler) stats(c *gin.Context) {
	u := userFrom(c)
	all := u.Role == store.RoleAdmin && c.Query("scope") == "all"
	stats, err := h.store.Stats(c.Request.Context(), u.ID, all)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load stats")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) formats(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	page, _, err := downloader.NormalizeURL(req.URL)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_url", "only YouTube video URLs are supported")
		return
	}
	ctx, cancel := contextWith(c, 90*time.Second)
	defer cancel()
	probe, err := h.runner.Probe(ctx, page)
	if err != nil {
		slog.Error("probe", "err", err)
		fail(c, http.StatusBadGateway, "ytdlp", publicErr(err))
		return
	}
	c.JSON(http.StatusOK, probe)
}

func (h *Handler) listDownloads(c *gin.Context) {
	u := userFrom(c)
	all := u.Role == store.RoleAdmin && c.Query("scope") == "all"
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			fail(c, http.StatusBadRequest, "bad_request", "invalid limit")
			return
		}
		if n > 500 {
			n = 500
		}
		limit = n
	}
	items, err := h.store.ListDownloads(c.Request.Context(), u.ID, all, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not list downloads")
		return
	}
	stats, err := h.store.Stats(c.Request.Context(), u.ID, all)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load stats")
		return
	}
	out := make([]gin.H, 0, len(items))
	for i := range items {
		out = append(out, present(&items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "stats": stats})
}

func (h *Handler) createDownload(c *gin.Context) {
	var req struct {
		URL     string `json:"url"`
		Kind    string `json:"kind"`
		Quality string `json:"quality"`
		Title   string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	page, videoID, err := downloader.NormalizeURL(req.URL)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_url", "only YouTube video URLs are supported")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	quality, err := downloader.NormalizeQuality(kind, req.Quality)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_quality", "kind must be video or audio, with a known quality")
		return
	}
	u := userFrom(c)
	existing, err := h.store.FindActive(c.Request.Context(), u.ID, page, kind, quality)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not check the queue")
		return
	}
	if existing != nil {
		c.JSON(http.StatusOK, gin.H{"download": present(existing), "duplicate": true})
		return
	}
	title := cleanTitle(req.Title)
	if title == "" {
		title = videoID
	}
	d := &store.Download{
		UserID:   u.ID,
		Username: u.Username,
		URL:      page,
		VideoID:  videoID,
		Title:    title,
		Kind:     kind,
		Quality:  quality,
	}
	if err := h.store.CreateDownload(c.Request.Context(), d); err != nil {
		slog.Error("create download", "err", err)
		fail(c, http.StatusInternalServerError, "internal", "could not queue download")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"download": present(d)})
}

func (h *Handler) getDownload(c *gin.Context) {
	d := h.owned(c)
	if d == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"download": present(d)})
}

func (h *Handler) pause(c *gin.Context) {
	d := h.owned(c)
	if d == nil {
		return
	}
	if d.Status != store.StatusDownloading && d.Status != store.StatusQueued {
		fail(c, http.StatusConflict, "conflict", "only a queued or active download can be paused")
		return
	}
	ctx, cancel := contextWith(c, 8*time.Second)
	defer cancel()
	if err := h.queue.Pause(ctx, d.ID); err != nil {
		slog.Error("pause", "id", d.ID, "err", err)
		fail(c, http.StatusConflict, "conflict", "could not pause download")
		return
	}
	h.writeDownload(c, d.ID)
}

func (h *Handler) resume(c *gin.Context) {
	d := h.owned(c)
	if d == nil {
		return
	}
	if d.Status != store.StatusPaused && d.Status != store.StatusFailed {
		fail(c, http.StatusConflict, "conflict", "only a paused or failed download can be resumed")
		return
	}
	if err := h.store.SetStatus(c.Request.Context(), d.ID, store.StatusQueued, "", []string{store.StatusPaused, store.StatusFailed}); err != nil {
		fail(c, http.StatusConflict, "conflict", "could not resume download")
		return
	}
	h.writeDownload(c, d.ID)
}

func (h *Handler) cancel(c *gin.Context) {
	d := h.owned(c)
	if d == nil {
		return
	}
	switch d.Status {
	case store.StatusQueued, store.StatusDownloading, store.StatusPaused, store.StatusFailed:
	default:
		fail(c, http.StatusConflict, "conflict", "this download cannot be cancelled")
		return
	}
	ctx, cancel := contextWith(c, 8*time.Second)
	defer cancel()
	if err := h.queue.Cancel(ctx, d.ID); err != nil {
		slog.Error("cancel", "id", d.ID, "err", err)
		fail(c, http.StatusConflict, "conflict", "could not cancel download")
		return
	}
	h.writeDownload(c, d.ID)
}

func (h *Handler) deleteDownload(c *gin.Context) {
	d := h.owned(c)
	if d == nil {
		return
	}
	ctx, cancel := contextWith(c, 8*time.Second)
	defer cancel()
	if d.Status == store.StatusDownloading || d.Status == store.StatusQueued || d.Status == store.StatusPaused {
		_ = h.queue.Cancel(ctx, d.ID)
	}
	h.removeFiles(d)
	if err := h.store.DeleteDownload(c.Request.Context(), d.ID); err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not delete download")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) downloadFile(c *gin.Context) {
	token := c.Param("token")
	if !fileTokenPattern.MatchString(token) {
		fail(c, http.StatusNotFound, "not_found", "file not found")
		return
	}
	d, err := h.store.GetByFileToken(c.Request.Context(), token)
	if err != nil || d.Status != store.StatusCompleted || d.FilePath == "" {
		fail(c, http.StatusNotFound, "not_found", "file not found")
		return
	}
	abs, err := securePath(h.cfg.DownloadDir, d.FilePath)
	if err != nil {
		fail(c, http.StatusNotFound, "not_found", "file not found")
		return
	}
	if _, err := os.Stat(abs); err != nil {
		fail(c, http.StatusNotFound, "not_found", "file not found")
		return
	}
	name := d.FileName
	if name == "" {
		name = filepath.Base(abs)
	}
	c.Header("Content-Disposition", contentDisposition(name))
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(abs)
}

func (h *Handler) createUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	name := strings.TrimSpace(req.Username)
	if strings.EqualFold(name, "admin") || !usernamePattern.MatchString(name) {
		fail(c, http.StatusBadRequest, "bad_request", "username must be 2-32 letters, numbers, _ or -")
		return
	}
	token, err := store.NewToken()
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not generate a key")
		return
	}
	u, err := h.store.CreateUser(c.Request.Context(), name, token)
	if err != nil {
		if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
			fail(c, http.StatusConflict, "conflict", "that username already exists")
			return
		}
		slog.Error("create user", "err", err)
		fail(c, http.StatusInternalServerError, "internal", "could not create user")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":         u.ID,
		"username":   u.Username,
		"role":       u.Role,
		"token":      u.Token,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) listUsers(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not list users")
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		out = append(out, gin.H{
			"id":         u.ID,
			"username":   u.Username,
			"role":       u.Role,
			"downloads":  u.Downloads,
			"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *Handler) deleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "bad_request", "invalid user id")
		return
	}
	u, err := h.store.UserByID(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		fail(c, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load user")
		return
	}
	if u.Role == store.RoleAdmin {
		fail(c, http.StatusForbidden, "forbidden", "the admin user cannot be deleted")
		return
	}
	items, err := h.store.ListAllByUser(c.Request.Context(), u.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not list user downloads")
		return
	}
	ctx, cancel := contextWith(c, 30*time.Second)
	defer cancel()
	for i := range items {
		d := &items[i]
		if d.Status == store.StatusDownloading || d.Status == store.StatusQueued || d.Status == store.StatusPaused {
			_ = h.queue.Cancel(ctx, d.ID)
		}
		h.removeFiles(d)
		if err := h.store.DeleteDownload(ctx, d.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusInternalServerError, "internal", "could not delete user downloads")
			return
		}
	}
	if err := h.store.DeleteUser(c.Request.Context(), u.ID); err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not delete user")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) getProxy(c *gin.Context) {
	value, _, err := h.store.GetSetting(c.Request.Context(), store.SettingDownloadProxy)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load proxy")
		return
	}
	c.JSON(http.StatusOK, gin.H{"proxy": value})
}

func (h *Handler) setProxy(c *gin.Context) {
	var req struct {
		Proxy string `json:"proxy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	proxy, err := downloader.NormalizeProxy(req.Proxy)
	if err != nil {
		fail(c, http.StatusBadRequest, "bad_proxy", err.Error())
		return
	}
	if err := h.store.SetSetting(c.Request.Context(), store.SettingDownloadProxy, proxy); err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not save proxy")
		return
	}
	if setter, ok := h.runner.(interface{ SetProxy(string) }); ok {
		setter.SetProxy(proxy)
	}
	if proxy == "" {
		slog.Info("download proxy cleared")
	} else {
		slog.Info("download proxy updated")
	}
	c.JSON(http.StatusOK, gin.H{"proxy": proxy})
}

func (h *Handler) ytdlpVersion(c *gin.Context) {
	ctx, cancel := contextWith(c, 60*time.Second)
	defer cancel()
	ver, err := h.runner.Version(ctx)
	if err != nil {
		fail(c, http.StatusBadGateway, "ytdlp", publicErr(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": ver})
}

func (h *Handler) updateYTDLP(c *gin.Context) {
	ctx, cancel := contextWith(c, 3*time.Minute)
	defer cancel()
	before, _ := h.runner.Version(ctx)
	out, err := h.runner.Update(ctx)
	after, _ := h.runner.Version(context.Background())
	if err != nil {
		slog.Error("ytdlp update", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  publicErr(err),
			"code":   "ytdlp",
			"before": before,
			"after":  after,
			"output": out,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"before": before, "after": after, "output": out})
}

func (h *Handler) owned(c *gin.Context) *store.Download {
	id := c.Param("id")
	if !downloadIDPattern.MatchString(id) {
		fail(c, http.StatusBadRequest, "bad_request", "invalid id")
		return nil
	}
	d, err := h.store.GetDownload(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		fail(c, http.StatusNotFound, "not_found", "download not found")
		return nil
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load download")
		return nil
	}
	u := userFrom(c)
	if u.Role != store.RoleAdmin && d.UserID != u.ID {
		fail(c, http.StatusNotFound, "not_found", "download not found")
		return nil
	}
	return d
}

func (h *Handler) writeDownload(c *gin.Context, id string) {
	d, err := h.store.GetDownload(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal", "could not load download")
		return
	}
	c.JSON(http.StatusOK, gin.H{"download": present(d)})
}

func (h *Handler) removeFiles(d *store.Download) {
	if d.FilePath != "" {
		if abs, err := securePath(h.cfg.DownloadDir, d.FilePath); err == nil {
			_ = os.Remove(abs)
		}
	}
	if downloadIDPattern.MatchString(d.ID) {
		_ = os.RemoveAll(filepath.Join(h.cfg.TempDir, d.ID))
	}
}

func present(d *store.Download) gin.H {
	item := gin.H{
		"id":               d.ID,
		"user_id":          d.UserID,
		"username":         d.Username,
		"file_token":       d.FileToken,
		"url":              d.URL,
		"video_id":         d.VideoID,
		"title":            d.Title,
		"kind":             d.Kind,
		"quality":          d.Quality,
		"status":           d.Status,
		"progress":         d.Progress,
		"bytes_downloaded": d.BytesDownloaded,
		"total_bytes":      d.TotalBytes,
		"speed_bps":        d.SpeedBPS,
		"file_size":        d.FileSize,
		"created_at":       stamp(d.CreatedAt),
		"updated_at":       stamp(d.UpdatedAt),
	}
	if d.ETA != "" {
		item["eta"] = d.ETA
	}
	if d.Error != "" {
		item["error"] = d.Error
	}
	if d.FileName != "" {
		item["file_name"] = d.FileName
	}
	if d.StartedAt != nil {
		item["started_at"] = stamp(*d.StartedAt)
	}
	if d.FinishedAt != nil {
		item["finished_at"] = stamp(*d.FinishedAt)
	}
	return item
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func userFrom(c *gin.Context) *store.User {
	v, _ := c.Get(userKey)
	u, _ := v.(*store.User)
	return u
}

func bearer(c *gin.Context) string {
	if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
		return key
	}
	h := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func fail(c *gin.Context, status int, code, msg string) {
	c.JSON(status, gin.H{"error": msg, "code": code})
}

func publicErr(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	r := []rune(msg)
	if len(r) > 400 {
		msg = string(r[len(r)-400:])
	}
	return msg
}

func cleanTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 300 {
		s = string([]rune(s)[:300])
	}
	return s
}

func securePath(root, target string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes download directory")
	}
	return targetAbs, nil
}

func contentDisposition(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`"\`, r) {
			return '_'
		}
		return r
	}, name)
	ascii := strings.Map(func(r rune) rune {
		if r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if ascii == "" {
		ascii = "download"
	}
	return `attachment; filename="` + ascii + `"; filename*=UTF-8''` + url.PathEscape(name)
}

func contextWith(c *gin.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), d)
}
