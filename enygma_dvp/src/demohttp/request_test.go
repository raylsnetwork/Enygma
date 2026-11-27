package demohttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckPost(t *testing.T) {
	for _, tc := range []struct {
		name, method, target, contentType string
		origins                           []string
		requireJSON                       bool
		want                              int
	}{
		{"same_origin_json", "POST", "http://localhost:9092/run", "application/json; charset=utf-8", []string{"http://localhost:9092"}, true, 200},
		{"same_origin_https", "POST", "https://demo.example/run", "", []string{"https://demo.example"}, false, 200},
		{"cli_without_origin", "POST", "http://127.0.0.1:9092/run", "application/json", nil, true, 200},
		{"post_only", "GET", "http://localhost:9092/run", "", nil, false, 405},
		{"foreign_origin", "POST", "http://localhost:9092/run", "application/json", []string{"http://localhost:9091"}, true, 403},
		{"wrong_scheme", "POST", "http://demo.example/run", "", []string{"https://demo.example"}, false, 403},
		{"opaque_origin", "POST", "http://localhost:9092/run", "", []string{"null"}, false, 403},
		{"multiple_origins", "POST", "http://localhost:9092/run", "", []string{"http://localhost:9092", "http://elsewhere.example"}, false, 403},
		{"non_json", "POST", "http://localhost:9092/run", "text/plain", nil, true, 415},
		{"missing_content_type", "POST", "http://localhost:9092/run", "", nil, true, 415},
		{"malformed_content_type", "POST", "http://localhost:9092/run", "application/json; charset=", nil, true, 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			for _, origin := range tc.origins {
				req.Header.Add("Origin", origin)
			}
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()
			if accepted := CheckPost(rec, req, tc.requireJSON); accepted != (tc.want == http.StatusOK) || rec.Code != tc.want {
				t.Fatalf("accepted = %t, status = %d, want %d", accepted, rec.Code, tc.want)
			}
		})
	}
}
