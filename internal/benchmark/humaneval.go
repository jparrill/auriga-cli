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
	formats.Register("humaneval", &HumanEvalRunner{})
}

type HumanEvalRunner struct{}

const humanEvalSystemPrompt = `You are an expert Python programmer. Complete the function below.
Output ONLY the function body (the code that goes after the function signature).
Do NOT include the function signature, imports, or any explanation.
Do NOT wrap the code in markdown code blocks.
Just output the raw Python code for the function body, properly indented.`

func (h *HumanEvalRunner) BuildPrompt(problem formats.Problem, suite formats.Suite) (string, error) {
	return fmt.Sprintf("%s\n\n%s", humanEvalSystemPrompt, problem.Prompt), nil
}

func (h *HumanEvalRunner) ValidateResponse(response string, problem formats.Problem, workDir string) (bool, string, error) {
	code := cleanPythonResponse(response, problem)

	// Combine: prompt (signature) + completion + test
	fullCode := problem.Prompt + code + "\n\n" + problem.Test + "\n"
	fullCode += fmt.Sprintf("\ncheck(%s)\n", problem.EntryPoint)

	testFile := filepath.Join(workDir, fmt.Sprintf("%s.py", sanitizeTaskID(problem.TaskID)))
	os.MkdirAll(workDir, 0755)
	os.WriteFile(testFile, []byte(fullCode), 0644)

	// Run with timeout
	cmd := goexec.Command("python3", testFile)
	cmd.Dir = workDir

	done := make(chan error, 1)
	var out []byte
	go func() {
		var e error
		out, e = cmd.CombinedOutput()
		done <- e
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		err = fmt.Errorf("timeout after 30s")
	}

	if err != nil {
		return false, fmt.Sprintf("test_fail: %s\n%s", err.Error(), truncateStr(string(out), 500)), nil
	}

	return true, "", nil
}

func (h *HumanEvalRunner) BuildRetryPrompt(problem formats.Problem, workDir string, validationError string) (string, error) {
	return fmt.Sprintf(`Your previous solution failed the tests. Error:
%s

Try again. Complete the function below.
Output ONLY the function body, no signature, no explanation, no markdown.

%s`, validationError, problem.Prompt), nil
}

func cleanPythonResponse(response string, problem formats.Problem) string {
	code := response

	// Strip markdown code blocks
	codeBlockRe := regexp.MustCompile("(?s)```(?:python)?\\s*\n(.*?)\n```")
	if matches := codeBlockRe.FindStringSubmatch(code); len(matches) > 1 {
		code = matches[1]
	}

	// Strip thinking blocks
	thinkRe := regexp.MustCompile("(?s)<think>.*?</think>")
	code = thinkRe.ReplaceAllString(code, "")

	code = strings.TrimSpace(code)

	// If the response includes the function signature, strip it
	if strings.Contains(code, "def "+problem.EntryPoint) {
		lines := strings.Split(code, "\n")
		inBody := false
		var bodyLines []string
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "def ") {
				inBody = true
				continue
			}
			if inBody {
				bodyLines = append(bodyLines, line)
			}
		}
		if len(bodyLines) > 0 {
			code = strings.Join(bodyLines, "\n")
		}
	}

	// Ensure proper indentation (4 spaces)
	lines := strings.Split(code, "\n")
	var indented []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			indented = append(indented, "")
		} else if !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "\t") {
			indented = append(indented, "    "+line)
		} else {
			indented = append(indented, line)
		}
	}

	return "\n" + strings.Join(indented, "\n") + "\n"
}

func sanitizeTaskID(id string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(id, "_")
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
