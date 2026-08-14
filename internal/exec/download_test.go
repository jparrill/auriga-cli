package exec

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"When size is in bytes, it should show bytes", 500, "500 B"},
		{"When size is in KB, it should show KB", 2048, "2 KB"},
		{"When size is in MB, it should show MB", 50 * 1024 * 1024, "50 MB"},
		{"When size is in GB, it should show GB with decimal", 7 * 1024 * 1024 * 1024, "7.0 GB"},
		{"When size is large GB, it should show correct value", 21_474_836_480, "20.0 GB"},
		{"When size is zero, it should show 0 B", 0, "0 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		name        string
		bytesPerSec float64
		want        string
	}{
		{"When speed is in B/s, it should show B/s", 500, "500 B/s"},
		{"When speed is in KB/s, it should show KB/s", 50 * 1024, "50 KB/s"},
		{"When speed is in MB/s, it should show MB/s", 52 * 1024 * 1024, "52 MB/s"},
		{"When speed is in GB/s, it should show GB/s", 1.5 * 1024 * 1024 * 1024, "1.5 GB/s"},
		{"When speed is zero, it should show 0 B/s", 0, "0 B/s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpeed(tt.bytesPerSec)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDownloadFile_Success(t *testing.T) {
	content := strings.Repeat("hello world ", 1000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "test.gguf")
	err := DownloadFile(context.Background(), server.URL+"/model.gguf", dest, "test.gguf", DownloadOpts{})

	if err != nil {
		t.Fatalf("When downloading a valid file, it should not error, got: %v", err)
	}

	data, _ := os.ReadFile(dest)
	if string(data) != content {
		t.Errorf("When downloading, file content should match server response")
	}
}

func TestDownloadFile_Resume(t *testing.T) {
	fullContent := "AAAAAABBBBBB"
	existing := "AAAAAA"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)-len(existing)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte(fullContent[len(existing):]))
		} else {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fullContent))
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "resume.gguf")
	os.WriteFile(dest, []byte(existing), 0644)

	err := DownloadFile(context.Background(), server.URL+"/model.gguf", dest, "resume.gguf", DownloadOpts{Resume: true})
	if err != nil {
		t.Fatalf("When resuming a download, it should not error, got: %v", err)
	}

	data, _ := os.ReadFile(dest)
	if string(data) != fullContent {
		t.Errorf("When resuming, final file should be full content, got %q", string(data))
	}
}

func TestDownloadFile_AlreadyComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "complete.gguf")
	os.WriteFile(dest, []byte("complete"), 0644)

	err := DownloadFile(context.Background(), server.URL+"/model.gguf", dest, "complete.gguf", DownloadOpts{Resume: true})
	if err != nil {
		t.Errorf("When file already complete (416), it should not error, got: %v", err)
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "missing.gguf")
	err := DownloadFile(context.Background(), server.URL+"/missing.gguf", dest, "missing.gguf", DownloadOpts{})

	if err == nil {
		t.Error("When server returns 404, it should return an error")
	}

	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("When download fails with HTTP error, file should not be created")
	}
}

func TestDownloadFile_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := filepath.Join(t.TempDir(), "cancelled.gguf")
	err := DownloadFile(ctx, server.URL+"/model.gguf", dest, "cancelled.gguf", DownloadOpts{})

	if err == nil {
		t.Error("When context is cancelled, it should return an error")
	}
}

func TestDownloadFile_NoResumeFlag_FreshDownload(t *testing.T) {
	content := "fresh content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			t.Error("When Resume is false, it should not send Range header")
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "fresh.gguf")
	os.WriteFile(dest, []byte("old content"), 0644)

	err := DownloadFile(context.Background(), server.URL+"/model.gguf", dest, "fresh.gguf", DownloadOpts{Resume: false})
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	data, _ := os.ReadFile(dest)
	if string(data) != content {
		t.Errorf("When Resume is false, it should overwrite existing file, got %q", string(data))
	}
}

func TestIsTTYOutput(t *testing.T) {
	got := isTTYOutput()
	_ = got
}

func TestClearLine_NoPanic(t *testing.T) {
	clearLine(true)
	clearLine(false)
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"When string fits, it should return as-is", "short", 10, "short"},
		{"When string equals max, it should return as-is", "exact", 5, "exact"},
		{"When string exceeds max, it should truncate with ellipsis", "very-long-filename.gguf", 15, "very-long-fi..."},
		{"When maxLen is very small, it should use minimum of 4", "abcdef", 2, "a..."},
		{"When string is empty, it should return empty", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetTermWidth_NoPanic(t *testing.T) {
	w := getTermWidth()
	if w <= 0 {
		t.Errorf("When getting terminal width, it should return positive value, got %d", w)
	}
}
