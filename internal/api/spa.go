package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// immutableAssets is the directory SvelteKit fills with content-hashed files;
// they can be cached forever.
const immutableAssets = "_app/immutable/"

// spa serves the embedded single-page app: real files as themselves, and every
// other path as the app shell, because routing happens in the browser.
func (h *Handler) spa(w http.ResponseWriter, r *http.Request) {
	if h.deps.SPA == nil {
		h.writeError(w, r, http.StatusServiceUnavailable,
			"this binary was built without the frontend; run `pnpm build` in web/ and rebuild")
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	info, err := fs.Stat(h.deps.SPA, name)
	if err != nil || info.IsDir() {
		// A client-side route, not a file: hand back the shell.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, h.deps.SPA, "index.html")
		return
	}

	if strings.HasPrefix(name, immutableAssets) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeFileFS(w, r, h.deps.SPA, name)
}
