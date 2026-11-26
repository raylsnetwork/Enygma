// Package proverhttp provides HTTP request guards for proving servers.
package proverhttp

import (
	"bytes"
	"io"
	"net/http"
)

// BufferBody checks the entire body against the limit before JSON decoding.
func BufferBody(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	limited := http.MaxBytesReader(w, r.Body, maxBytes)
	defer limited.Close()
	body, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}
