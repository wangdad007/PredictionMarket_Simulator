package ipfs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploaderSendsMetadataAndReturnsContentCID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"desc":"黄金 上涨"}` {
			t.Fatalf("uploaded metadata = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Hash":"local-v1-abc","Name":"market.json"}`))
	}))
	defer server.Close()

	cid, err := NewUploader(server.URL).UploadMetadata(context.Background(), `{"desc":"黄金 上涨"}`)
	if err != nil {
		t.Fatal(err)
	}
	if cid != "local-v1-abc" {
		t.Fatalf("cid = %q, want local-v1-abc", cid)
	}
}

func TestUploaderReportsUploadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "content service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := NewUploader(server.URL).UploadMetadata(context.Background(), `{"desc":"黄金"}`)
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("upload error = %v, want HTTP 503", err)
	}
}
