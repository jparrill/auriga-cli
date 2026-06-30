package benchmark

import (
	"fmt"
	goexec "os/exec"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
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
	var cmd *goexec.Cmd

	// Detect build command based on files present
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		cmd = goexec.Command("go", "build", "./...")
	} else if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		cmd = goexec.Command("npm", "run", "build")
	} else if _, err := os.Stat(filepath.Join(workDir, "Makefile")); err == nil {
		cmd = goexec.Command("make", "build")
	} else {
		// Try go build on any .go files
		matches, _ := filepath.Glob(filepath.Join(workDir, "*.go"))
		if len(matches) > 0 {
			cmd = goexec.Command("go", "build", "./...")
		} else {
			return true, "" // No build system detected, skip
		}
	}

	cmd.Dir = workDir
	timer := time.AfterFunc(60*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	out, err := cmd.CombinedOutput()
	timer.Stop()

	if err != nil {
		return false, truncateStr(string(out), 1500)
	}
	return true, ""
}

func runTestCheck(workDir string, problem formats.Problem) (bool, string) {
	var cmd *goexec.Cmd

	if problem.TestCmd != "" {
		parts := strings.Fields(problem.TestCmd)
		cmd = goexec.Command(parts[0], parts[1:]...)
	} else if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		cmd = goexec.Command("go", "test", "./...")
	} else if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		cmd = goexec.Command("npm", "test")
	} else {
		// Find *_test.go files
		matches, _ := filepath.Glob(filepath.Join(workDir, "*_test.go"))
		if len(matches) > 0 {
			cmd = goexec.Command("go", "test", "./...")
		} else {
			return true, "" // No tests found, skip
		}
	}

	cmd.Dir = workDir
	timer := time.AfterFunc(120*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	out, err := cmd.CombinedOutput()
	timer.Stop()

	if err != nil {
		// Extract test failures
		output := string(out)
		failRe := regexp.MustCompile(`(?m)^--- FAIL.*$`)
		failures := failRe.FindAllString(output, -1)
		summary := truncateStr(output, 1500)
		if len(failures) > 0 {
			summary = strings.Join(failures, "\n") + "\n\n" + summary
		}
		return false, summary
	}
	return true, ""
}
