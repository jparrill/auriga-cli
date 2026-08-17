package huggingface

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	goexec "os/exec"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/ui"
)

type LFSInfo struct {
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}

type RepoFile struct {
	Path string   `json:"path"`
	Size int64    `json:"size"`
	LFS  *LFSInfo `json:"lfs,omitempty"`
}

func resolveToken() string {
	if t := os.Getenv("HF_TOKEN"); t != "" {
		return t
	}
	out, err := goexec.Command("pass", "HuggingFace/auriga-token").Output()
	if err == nil {
		token := strings.TrimSpace(string(out))
		if token != "" {
			return token
		}
	}
	return ""
}

func ListFiles(repo string) ([]RepoFile, error) {
	url := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", repo)
	token := resolveToken()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			ui.Logger.Debug("HF retry", "attempt", attempt+1, "backoff", backoff)
			time.Sleep(backoff)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HF API error for %s: %w", repo, err)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HF API returned %d for %s", resp.StatusCode, repo)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("HF API returned %d for %s", resp.StatusCode, repo)
		}

		var files []RepoFile
		err = json.NewDecoder(resp.Body).Decode(&files)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("invalid HF response: %w", err)
		}
		return files, nil
	}

	return nil, lastErr
}

func ResolveGGUF(repo string, quantPriority []string) (string, int64, error) {
	files, err := ListFiles(repo)
	if err != nil {
		return "", 0, err
	}

	var ggufFiles []RepoFile
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".gguf") && !strings.Contains(strings.ToLower(f.Path), "mmproj") {
			ggufFiles = append(ggufFiles, f)
		}
	}

	for _, q := range quantPriority {
		for _, gf := range ggufFiles {
			if strings.Contains(gf.Path, "UD-"+q) {
				return gf.Path, gf.Size, nil
			}
		}
	}
	for _, q := range quantPriority {
		for _, gf := range ggufFiles {
			if strings.Contains(gf.Path, q) {
				return gf.Path, gf.Size, nil
			}
		}
	}

	for _, gf := range ggufFiles {
		if gf.Size > 1_000_000_000 {
			return gf.Path, gf.Size, nil
		}
	}

	if len(ggufFiles) > 0 {
		return ggufFiles[0].Path, ggufFiles[0].Size, nil
	}
	return "", 0, fmt.Errorf("no GGUF found in %s", repo)
}

func ResolveMMProj(repo string) (string, int64, error) {
	files, err := ListFiles(repo)
	if err != nil {
		return "", 0, err
	}

	for _, f := range files {
		lower := strings.ToLower(f.Path)
		if strings.Contains(lower, "mmproj") && strings.HasSuffix(lower, ".gguf") {
			return f.Path, f.Size, nil
		}
	}

	return "", 0, fmt.Errorf("no mmproj found in %s", repo)
}

func ResolveDFlash(repo string) (string, int64, error) {
	files, err := ListFiles(repo)
	if err != nil {
		return "", 0, err
	}

	for _, f := range files {
		lower := strings.ToLower(f.Path)
		if strings.Contains(lower, "dflash") && strings.HasSuffix(lower, ".gguf") {
			return f.Path, f.Size, nil
		}
	}

	return "", 0, fmt.Errorf("no dflash drafter found in %s", repo)
}

func DownloadURL(repo, filename string) string {
	return fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, filename)
}

func ExpectedSize(repo, filename string) (int64, error) {
	files, err := ListFiles(repo)
	if err != nil {
		return 0, err
	}
	for _, f := range files {
		if f.Path == filename {
			return f.Size, nil
		}
	}
	return 0, fmt.Errorf("file %s not found in %s", filename, repo)
}

func ExpectedHash(repo, filename string) (hash string, size int64, err error) {
	files, err := ListFiles(repo)
	if err != nil {
		return "", 0, err
	}
	for _, f := range files {
		if f.Path == filename {
			if f.LFS != nil && f.LFS.OID != "" {
				return f.LFS.OID, f.LFS.Size, nil
			}
			return "", f.Size, nil
		}
	}
	return "", 0, fmt.Errorf("file %s not found in %s", filename, repo)
}
