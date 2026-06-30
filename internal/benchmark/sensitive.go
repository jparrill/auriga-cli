package benchmark

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"gopkg.in/yaml.v3"
)

type Violation struct {
	Description string
	FilePath    string
}

var sensitivePatterns []struct {
	Pattern     *regexp.Regexp
	Description string
}

func init() {
	LoadSensitivePatterns()
}

func LoadSensitivePatterns() {
	configFile := config.ExpandHome("~/.config/auriga/sensitive-patterns.yaml")
	data, err := os.ReadFile(configFile)
	if err != nil {
		// Default: no patterns (user must configure)
		return
	}

	var entries []struct {
		Pattern     string `yaml:"pattern"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return
	}

	sensitivePatterns = nil
	for _, e := range entries {
		re, err := regexp.Compile(e.Pattern)
		if err != nil {
			continue
		}
		sensitivePatterns = append(sensitivePatterns, struct {
			Pattern     *regexp.Regexp
			Description string
		}{re, e.Description})
	}
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
