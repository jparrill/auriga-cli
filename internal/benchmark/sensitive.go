package benchmark

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Violation struct {
	Description string
	FilePath    string
}

var sensitivePatterns = []struct {
	Pattern     *regexp.Regexp
	Description string
}{
	{regexp.MustCompile(`192\.168\.1\.143`), "Server LAN IP"},
	{regexp.MustCompile(`192\.168\.1\.117`), "Server old IP"},
	{regexp.MustCompile(`100\.77\.65\.108`), "Tailscale IP (server)"},
	{regexp.MustCompile(`100\.108\.82\.122`), "Tailscale IP (Mac)"},
	{regexp.MustCompile(`itpc-gcp-hcm-pe-eng-claude`), "Vertex AI project ID"},
	{regexp.MustCompile(`8648704793`), "Telegram bot token prefix"},
	{regexp.MustCompile(`AAFe8izbLr5uwh57k9ZyenJRcRUPJZ_vBnA`), "Telegram bot token"},
	{regexp.MustCompile(`30890766`), "Telegram chat ID"},
	{regexp.MustCompile(`6ScAPZK7`), "Odysseus password prefix"},
	{regexp.MustCompile(`jparrill@redhat\.com`), "Work email"},
	{regexp.MustCompile(`padajuan@gmail\.com`), "Personal email"},
	{regexp.MustCompile(`BA202938CB1C0C1E251F966ADE30627E53AC3969`), "GPG fingerprint"},
	{regexp.MustCompile(`xenomorph`), "Mac hostname"},
	{regexp.MustCompile(`22E00A391800001`), "Printer serial number"},
}

var checkExts = map[string]bool{
	".astro": true, ".json": true, ".js": true, ".ts": true,
	".css": true, ".md": true, ".html": true, ".mjs": true, ".txt": true,
}

func CheckSensitiveData(directory string) []Violation {
	var violations []Violation

	filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !checkExts[ext] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		relPath, _ := filepath.Rel(directory, path)

		for _, sp := range sensitivePatterns {
			if sp.Pattern.MatchString(content) {
				violations = append(violations, Violation{
					Description: sp.Description,
					FilePath:    relPath,
				})
			}
		}
		return nil
	})

	return violations
}
