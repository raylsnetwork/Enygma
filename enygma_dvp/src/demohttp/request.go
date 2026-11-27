// Package demohttp contains HTTP request checks shared by the local demos.
package demohttp

import (
	"mime"
	"net/http"
)

// CheckPost accepts same-origin browser requests and CLI requests without an
// Origin header. Routes that decode JSON also require an application/json body.
func CheckPost(w http.ResponseWriter, r *http.Request, requireJSON bool) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	origins := r.Header.Values("Origin")
	if len(origins) > 0 && (len(origins) != 1 || origins[0] != scheme+"://"+r.Host) {
		http.Error(w, "request must originate from this demo", http.StatusForbidden)
		return false
	}

	if requireJSON {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return false
		}
	}
	return true
}
