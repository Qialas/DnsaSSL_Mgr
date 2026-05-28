package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterServesEmbeddedAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	root := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(root, req)
	if root.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", root.Code, http.StatusOK)
	}
	if !strings.Contains(root.Body.String(), `<div id="root">`) {
		t.Fatalf("GET / did not return admin index")
	}

	matches, err := fs.Glob(distFS, "dist/assets/*.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no embedded js assets found")
	}

	assetPath := strings.TrimPrefix(matches[0], "dist")
	asset := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, assetPath, nil)
	r.ServeHTTP(asset, req)
	if asset.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", assetPath, asset.Code, http.StatusOK)
	}
}
