package benchmark

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/spf13/viper"
)

//go:embed prompts/*.md
var embeddedPrompts embed.FS

type AffectedFile struct {
	Path    string
	Content string
}

type SensitiveRetryData struct {
	Violations    []Violation
	AffectedFiles []AffectedFile
}

type BuildRetryData struct {
	Error         string
	AffectedFiles []AffectedFile
}

func loadPromptTemplate(name string) (string, error) {
	// 1. Try user override: ~/.config/auriga/prompts/<name>
	promptsDir := config.ExpandHome(viper.GetString("prompts.dir"))
	if promptsDir == "" {
		promptsDir = config.ExpandHome("~/.config/auriga/prompts")
	}
	userPath := filepath.Join(promptsDir, name)
	if data, err := os.ReadFile(userPath); err == nil {
		return string(data), nil
	}

	// 2. Fallback: embedded
	data, err := embeddedPrompts.ReadFile("prompts/" + name)
	if err != nil {
		return "", fmt.Errorf("prompt template %q not found", name)
	}
	return string(data), nil
}

func renderTemplate(name string, data interface{}) (string, error) {
	tmplStr, err := loadPromptTemplate(name)
	if err != nil {
		return "", err
	}

	if data == nil {
		return tmplStr, nil
	}

	tmpl, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("cannot parse template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("cannot render template %q: %w", name, err)
	}
	return buf.String(), nil
}

func BuildPrompt(planFile, sourceHTML, benchmarksJSON string) (string, error) {
	system, err := renderTemplate("system.md", nil)
	if err != nil {
		return "", err
	}

	plan, err := os.ReadFile(planFile)
	if err != nil {
		return "", fmt.Errorf("cannot read plan: %w", err)
	}

	source, err := os.ReadFile(sourceHTML)
	if err != nil {
		return "", fmt.Errorf("cannot read source HTML: %w", err)
	}
	sourceStr := string(source)
	if len(sourceStr) > 50000 {
		sourceStr = sourceStr[:50000]
	}

	benchmarks, err := os.ReadFile(benchmarksJSON)
	if err != nil {
		return "", fmt.Errorf("cannot read benchmarks: %w", err)
	}

	return fmt.Sprintf("%s\n\n=== PROJECT PLAN ===\n%s\n\n=== SOURCE HTML ===\n%s\n\n=== BENCHMARK DATA ===\n%s\n\nGenerate the complete project now.",
		system, string(plan), sourceStr, string(benchmarks)), nil
}

func BuildFormatRetryPrompt(originalPrompt string) string {
	tmpl, err := renderTemplate("format-retry.md", nil)
	if err != nil {
		return originalPrompt
	}
	return tmpl + originalPrompt
}

func BuildSensitiveRetryPrompt(projectDir string, violations []Violation) (string, error) {
	var affected []AffectedFile
	seen := make(map[string]bool)
	for _, v := range violations {
		if seen[v.FilePath] {
			continue
		}
		seen[v.FilePath] = true
		content, err := os.ReadFile(filepath.Join(projectDir, v.FilePath))
		if err != nil {
			continue
		}
		affected = append(affected, AffectedFile{
			Path:    v.FilePath,
			Content: string(content),
		})
	}

	data := SensitiveRetryData{
		Violations:    violations,
		AffectedFiles: affected,
	}

	return renderTemplate("sensitive-retry.md", data)
}

func BuildBuildRetryPrompt(projectDir, buildError string) (string, error) {
	var affected []AffectedFile

	// Include files most likely to cause build errors
	candidates := []string{
		"package.json",
		"astro.config.mjs",
		"tsconfig.json",
	}

	for _, c := range candidates {
		path := filepath.Join(projectDir, c)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		affected = append(affected, AffectedFile{
			Path:    c,
			Content: string(content),
		})
	}

	// Also include files mentioned in the error
	for _, line := range strings.Split(buildError, "\n") {
		for _, ext := range []string{".astro", ".js", ".mjs", ".ts", ".json"} {
			if idx := strings.Index(line, ext); idx > 0 {
				start := strings.LastIndexAny(line[:idx], " '\"(/") + 1
				fpath := line[start : idx+len(ext)]
				fpath = strings.TrimPrefix(fpath, "/")

				fullPath := filepath.Join(projectDir, fpath)
				if _, err := os.Stat(fullPath); err == nil {
					already := false
					for _, a := range affected {
						if a.Path == fpath {
							already = true
							break
						}
					}
					if !already {
						content, _ := os.ReadFile(fullPath)
						affected = append(affected, AffectedFile{
							Path:    fpath,
							Content: string(content),
						})
					}
				}
			}
		}
	}

	data := BuildRetryData{
		Error:         buildError,
		AffectedFiles: affected,
	}

	return renderTemplate("build-retry.md", data)
}
