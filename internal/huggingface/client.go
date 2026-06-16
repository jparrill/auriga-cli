package huggingface

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RepoFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func ListFiles(repo string) ([]RepoFile, error) {
	url := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", repo)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HF API error for %s: %w", repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HF API returned %d for %s", resp.StatusCode, repo)
	}

	var files []RepoFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("invalid HF response: %w", err)
	}
	return files, nil
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

func DownloadURL(repo, filename string) string {
	return fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, filename)
}
