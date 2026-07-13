package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
	"github.com/jparrill/auriga-cli/internal/exec"
)

func init() {
	formats.Register("quality", &QualityRunner{})
}

type QualityRunner struct{}

const qualitySystemPrompt = `You are an expert software engineer. Complete the task below.
Output ONLY the code files needed, using this format for each file:

--- FILE: path/to/file.ext ---
(file content)
--- END FILE ---

Rules:
- Every file must be complete and compilable
- Include proper error handling
- Follow idiomatic patterns for the language
- If tests are requested, include them
- Do NOT include explanations outside of file blocks`

func (q *QualityRunner) BuildPrompt(problem formats.Problem, suite formats.Suite) (string, error) {
	return fmt.Sprintf("%s\n\nLanguage: %s\nDifficulty: %s\n\n=== TASK ===\n%s",
		qualitySystemPrompt, suite.Language, problem.Level, problem.Prompt), nil
}

func (q *QualityRunner) ValidateResponse(response string, problem formats.Problem, workDir string) (bool, string, error) {
	// Parse files from response
	parsed, _ := ParseFiles(response, workDir)
	if parsed == 0 {
		return false, "no_files", nil
	}

	// Run evaluations specified in problem
	for _, eval := range problem.Eval {
		switch eval {
		case "build":
			ok, errMsg := runBuildCheck(workDir, problem)
			if !ok {
				return false, "build_fail:" + errMsg, nil
			}
		case "test":
			ok, errMsg := runTestCheck(workDir, problem)
			if !ok {
				return false, "test_fail:" + errMsg, nil
			}
		}
	}

	return true, "", nil
}

func (q *QualityRunner) BuildRetryPrompt(problem formats.Problem, workDir string, validationError string) (string, error) {
	if strings.HasPrefix(validationError, "no_files") {
		return BuildFormatRetryPrompt(""), nil
	}

	// Collect relevant files from workDir
	var files []string
	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(workDir, path)
			content, _ := os.ReadFile(path)
			files = append(files, fmt.Sprintf("--- CURRENT FILE: %s ---\n%s\n--- END CURRENT FILE ---", rel, string(content)))
		}
		return nil
	})

	errorType := "Build"
	errorDetail := validationError
	if strings.HasPrefix(validationError, "build_fail:") {
		errorDetail = validationError[11:]
	} else if strings.HasPrefix(validationError, "test_fail:") {
		errorType = "Test"
		errorDetail = validationError[10:]
	}

	return fmt.Sprintf(`%s failed with error:

%s

Current files:
%s

Fix the issue and regenerate ONLY the files that need to change.
Use --- FILE: path --- / --- END FILE --- format.
Do NOT regenerate files that are not related to the error.

Original task:
%s`, errorType, errorDetail, strings.Join(files, "\n\n"), problem.Prompt), nil
}

func runBuildCheck(workDir string, problem formats.Problem) (bool, string) {
	var name string
	var args []string
	var image string

	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		name, args, image = "go", []string{"build", "./..."}, exec.ImageGo
	} else if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		name, args, image = "npm", []string{"run", "build"}, exec.ImageNode
	} else if _, err := os.Stat(filepath.Join(workDir, "Makefile")); err == nil {
		name, args, image = "make", []string{"build"}, exec.ImageGo
	} else {
		matches, _ := filepath.Glob(filepath.Join(workDir, "*.go"))
		if len(matches) > 0 {
			name, args, image = "go", []string{"build", "./..."}, exec.ImageGo
		} else {
			return true, ""
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := exec.RunSandboxed(ctx, name, args, exec.SandboxOpts{Dir: workDir, Image: image})
	if err != nil {
		return false, truncateStr(out, 1500)
	}
	return true, ""
}

func runTestCheck(workDir string, problem formats.Problem) (bool, string) {
	var name string
	var args []string
	var image string

	if problem.TestCmd != "" {
		parts := strings.Fields(problem.TestCmd)
		name, args = parts[0], parts[1:]
		image = exec.ImageGo
	} else if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		name, args, image = "go", []string{"test", "./..."}, exec.ImageGo
	} else if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		name, args, image = "npm", []string{"test"}, exec.ImageNode
	} else {
		matches, _ := filepath.Glob(filepath.Join(workDir, "*_test.go"))
		if len(matches) > 0 {
			name, args, image = "go", []string{"test", "./..."}, exec.ImageGo
		} else {
			return true, ""
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	out, err := exec.RunSandboxed(ctx, name, args, exec.SandboxOpts{Dir: workDir, Image: image})
	if err != nil {
		failRe := regexp.MustCompile(`(?m)^--- FAIL.*$`)
		failures := failRe.FindAllString(out, -1)
		summary := truncateStr(out, 1500)
		if len(failures) > 0 {
			summary = strings.Join(failures, "\n") + "\n\n" + summary
		}
		return false, summary
	}
	return true, ""
}
