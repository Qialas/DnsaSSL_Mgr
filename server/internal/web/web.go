package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist
var distFS embed.FS

func Register(r *gin.Engine) {
	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(staticFS))
	r.GET("/", serveIndex(staticFS))
	r.GET("/assets/*filepath", gin.WrapH(http.StripPrefix("/assets/", http.FileServer(http.FS(mustSub(staticFS, "assets"))))))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "接口不存在"})
			return
		}
		if exists(staticFS, strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")) {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		serveIndexFile(c, staticFS)
	})
}

func serveIndex(staticFS fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		serveIndexFile(c, staticFS)
	}
}

func serveIndexFile(c *gin.Context, staticFS fs.FS) {
	data, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "admin web not found")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func exists(staticFS fs.FS, name string) bool {
	if name == "." || name == "" {
		return false
	}
	info, err := fs.Stat(staticFS, name)
	return err == nil && !info.IsDir()
}

func mustSub(staticFS fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(staticFS, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
