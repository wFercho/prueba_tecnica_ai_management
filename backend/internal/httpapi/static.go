package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// dashboardFallback answers the paths the API does not claim: the built dashboard's
// own files, and the client-side routes that have no file behind them.
type dashboardFallback struct {
	index  string
	files  http.Handler
	absent bool
	log    *slog.Logger
}

// withDashboard teaches the handler to serve the built dashboard for the paths it
// does not route itself.
//
// The API's routes always win: ServeMux reports which pattern a request matched, and
// anything that matched nothing is the dashboard's to answer. Registering the fallback
// the other way round — wrapping the API — would let the page swallow /meters/M-109,
// which is the shape of path the dashboard's own navigation uses.
//
// A missing build is not an error. `go run ./cmd/server` with no frontend build
// serves the API and logs why there is no page, rather than refusing to start.
func (h *Handler) withDashboard(dir string, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		log.Info("no built dashboard found, serving the API only",
			"looked_in", dir, "build_it_with", "make frontend")
		h.dashboard = nil
		return nil
	}
	h.dashboard = &dashboardFallback{
		index: index,
		files: http.FileServer(http.Dir(dir)),
		log:   log,
	}
	return nil
}

// ServeDashboard serves the built dashboard from the API's own origin, so production
// has one origin, no CORS headers and one thing to deploy. In development the Vite
// dev server proxies to the API instead, which the browser cannot tell apart.
func (h *Handler) ServeDashboard(dir string, log *slog.Logger) error {
	return h.withDashboard(dir, log)
}

func (d *dashboardFallback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))

	if _, err := os.Stat(filepath.Join(filepath.Dir(d.index), filepath.FromSlash(name))); err == nil {
		d.files.ServeHTTP(w, r)
		return
	}
	// A missing asset is a build problem, and answering it with index.html turns a 404
	// in the console into a module that fails to parse far from the cause. Only a
	// path that could be a client-side route falls back to the page.
	if looksLikeAsset(name) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, d.index)
}

// looksLikeAsset reports whether a path names a file the build produced, as opposed
// to a route the dashboard navigates to.
func looksLikeAsset(name string) bool {
	if strings.HasPrefix(name, "/assets/") {
		return true
	}
	switch path.Ext(name) {
	case ".js", ".mjs", ".css", ".map", ".json", ".svg", ".png", ".jpg", ".jpeg",
		".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf":
		return true
	default:
		return false
	}
}
