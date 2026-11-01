// Package gui serves the desktop window and opens it in a local browser.
package gui

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/gin-gonic/gin"
)

//go:embed assets/index.html assets/app.css assets/app.js
var files embed.FS

// Deps is the local-only state the window needs.
type Deps struct {
	Key         string
	DownloadDir string
	Quit        func()
	Open        func(dir string) error
}

// Mount adds /app on an existing API router. GET / stays the JSON root.
func Mount(r *gin.Engine, deps Deps) {
	if deps.Open == nil {
		deps.Open = openFolder
	}
	if deps.Quit == nil {
		deps.Quit = func() {}
	}
	h := handler{deps: deps, page: template.Must(template.New("index.html").ParseFS(files, "assets/index.html"))}
	r.GET("/app", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/app/")
	})
	r.GET("/app/", h.pageHandler)
	r.GET("/app/app.js", staticFile("assets/app.js", "text/javascript; charset=utf-8"))
	r.GET("/app/app.css", staticFile("assets/app.css", "text/css; charset=utf-8"))
	r.POST("/app/quit", h.quit)
	r.POST("/app/folder", h.folder)
}

type handler struct {
	deps Deps
	page *template.Template
}

type pageData struct {
	Boot template.JS
}

func (h handler) pageHandler(c *gin.Context) {
	key := ""
	// RemoteAddr is the TCP peer. A forwarded header cannot mark a remote client as local.
	if loopback(c.Request) {
		key = h.deps.Key
	}
	// template.JS leaves this object literal intact. json.Marshal escapes < so it cannot close the script tag.
	raw, err := json.Marshal(struct {
		Key string `json:"key"`
	}{Key: key})
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := h.page.Execute(&buf, pageData{Boot: template.JS(raw)}); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (h handler) quit(c *gin.Context) {
	if !loopback(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "local only"})
		return
	}
	c.Status(http.StatusNoContent)
	h.deps.Quit()
}

func (h handler) folder(c *gin.Context) {
	if !loopback(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "local only"})
		return
	}
	// The folder is the app download directory. The request body is ignored.
	if c.Request.Body != nil {
		_, _ = io.Copy(io.Discard, c.Request.Body)
	}
	if err := h.deps.Open(h.deps.DownloadDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not open the downloads folder"})
		return
	}
	c.Status(http.StatusNoContent)
}

func staticFile(name, contentType string) gin.HandlerFunc {
	body, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, contentType, body)
	}
}

func loopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func openFolder(dir string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}
