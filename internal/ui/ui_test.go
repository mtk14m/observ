package ui_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mtk14n/obsrv/internal/ui"
)

func serve(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	b, _ := io.ReadAll(rec.Body)
	return rec.Code, string(b)
}

var built = fstest.MapFS{
	"index.html":       {Data: []byte("<html>app</html>")},
	"assets/app-1.js":  {Data: []byte("console.log(1)")},
	"assets/app-1.css": {Data: []byte("body{}")},
}

func TestServesIndexAndAssets(t *testing.T) {
	h := ui.NewHandler(built)
	if code, body := serve(t, h, "/"); code != http.StatusOK || body != "<html>app</html>" {
		t.Errorf("/ = %d %q", code, body)
	}
	if code, body := serve(t, h, "/assets/app-1.js"); code != http.StatusOK || body != "console.log(1)" {
		t.Errorf("asset = %d %q", code, body)
	}
}

func TestClientRoutesFallBackToIndex(t *testing.T) {
	h := ui.NewHandler(built)
	for _, path := range []string{"/logs", "/traces/0af7651916cd43dd8448eb211c80319c"} {
		if code, body := serve(t, h, path); code != http.StatusOK || body != "<html>app</html>" {
			t.Errorf("%s = %d %q, want index.html", path, code, body)
		}
	}
}

func TestMissingAssetsAre404(t *testing.T) {
	if code, _ := serve(t, ui.NewHandler(built), "/assets/gone.js"); code != http.StatusNotFound {
		t.Errorf("missing asset = %d, want 404", code)
	}
}

func TestPlaceholderWhenUIIsNotBuilt(t *testing.T) {
	code, body := serve(t, ui.NewHandler(fstest.MapFS{}), "/")
	if code != http.StatusOK || !strings.Contains(body, "make web-build") {
		t.Errorf("placeholder = %d %q", code, body)
	}
}
