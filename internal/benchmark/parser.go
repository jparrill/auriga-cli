package benchmark

import (
	"os"
	"path/filepath"
	"regexp"
)

var (
	strictPattern   = regexp.MustCompile(`(?s)--- FILE: (.+?) ---\n(.*?)--- END FILE ---`)
	backtickPattern = regexp.MustCompile("(?s)--- FILE: (.+?) ---\\s*\n```[^\\n]*\\n(.*?)\\n```\\s*\\n(?:--- END FILE ---)?")

)

func ParseFiles(rawOutput, targetDir string) (int, error) {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, err
	}

	// Try backtick format first (more specific), then strict
	matches := backtickPattern.FindAllStringSubmatch(rawOutput, -1)
	if len(matches) == 0 {
		matches = strictPattern.FindAllStringSubmatch(rawOutput, -1)
	}

	if len(matches) == 0 {
		os.WriteFile(filepath.Join(targetDir, "_raw_output.txt"), []byte(rawOutput), 0644)
		return 0, nil
	}

	for _, m := range matches {
		fpath := filepath.Join(targetDir, m[1])
		dir := filepath.Dir(fpath)
		os.MkdirAll(dir, 0755)
		content := m[2]
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content += "\n"
		}
		if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
			continue
		}
	}

	return len(matches), nil
}
