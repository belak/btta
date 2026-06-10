package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestDownloadMedia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/media/ok.png" {
			_, _ = w.Write([]byte("PNGDATA"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	tests := []struct {
		name        string
		base        string
		ref         string
		wantErr     string // substring; "" means no error
		wantName    string
		wantContent string
	}{
		{name: "same host downloads", base: srv.URL, ref: "/media/ok.png", wantName: "ok.png", wantContent: "PNGDATA"},
		{name: "cross host refused", base: srv.URL, ref: "http://169.254.169.254/latest/meta-data/", wantErr: "refusing media from host"},
		{name: "non-http refused", base: "http://example.com", ref: "file:///etc/passwd", wantErr: "non-http(s)"},
		{name: "empty ref is a no-op", base: "http://example.com", ref: "", wantName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			name, err := downloadMedia(context.Background(), tt.base, tt.ref, dir)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
			if tt.wantContent != "" {
				got, err := os.ReadFile(filepath.Join(dir, name))
				assert.NoError(t, err)
				assert.Equal(t, tt.wantContent, string(got))
			}
		})
	}
}
