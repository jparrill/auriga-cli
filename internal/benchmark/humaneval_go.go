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
	formats.Register("humaneval-go", &HumanEvalGoRunner{})
}

type HumanEvalGoRunner struct{}

const humanEvalGoSystemPrompt = `You are an expert Go programmer. Complete the function below.
Output ONLY the function body (the code that goes after the function signature).
Do NOT include the function signature, imports, package declaration, or any explanation.
Do NOT wrap the code in markdown code blocks.
Just output the raw Go code for the function body, properly indented.`

func (h *HumanEvalGoRunner) BuildPrompt(problem formats.Problem, suite formats.Suite) (string, error) {
	return fmt.Sprintf("%s\n\n%s", humanEvalGoSystemPrompt, problem.Prompt), nil
}

func (h *HumanEvalGoRunner) ValidateResponse(response string, problem formats.Problem, workDir string) (bool, string, error) {
	code := cleanGoResponse(response, problem)

	fullCode := buildGoTestFile(problem, code)

	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "solution_test.go")
	os.WriteFile(testFile, []byte(fullCode), 0644)

	goMod := "module solution\n\ngo 1.22\n\nrequire github.com/stretchr/testify v1.9.0\n\nrequire (\n\tgithub.com/davecgh/go-spew v1.1.1 // indirect\n\tgithub.com/pmezard/go-difflib v1.0.0 // indirect\n\tgithub.com/stretchr/objx v0.5.2 // indirect\n\tgopkg.in/yaml.v3 v3.0.1 // indirect\n)\n"
	os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goMod), 0644)

	goSum := `github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77E/DIRtfOGp7rxMxcFuKD9Tt6pToFo5MfzbppSQ=
github.com/stretchr/objx v0.5.2 h1:xuMeJ0Sdp5ZMRXx/aWO6RZxdr3beISkG5/G/aIRr3pY=
github.com/stretchr/objx v0.5.2/go.mod h1:pMhQbd1rxFRPy+5PCCU+MczzoTm/2dNs+t5aJIolccM=
github.com/stretchr/testify v1.9.0 h1:HtqpIVDClZ4nwg75+f6Lvsy/wHu+3BoSGCbBAcpTsTg=
github.com/stretchr/testify v1.9.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405 h1:yhCVgyC4o1EVCAEJBkOfkSuQQ1skiXFQoGk/Y2O6UJo=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Gy0=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJE68CQ=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
`
	os.WriteFile(filepath.Join(workDir, "go.sum"), []byte(goSum), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := exec.RunSandboxed(ctx, "go", []string{"test", "-v", "-count=1", "./..."}, exec.SandboxOpts{
		Dir:   workDir,
		Image: exec.ImageGo,
	})

	if err != nil {
		return false, fmt.Sprintf("test_fail: %s\n%s", err.Error(), truncateStr(out, 500)), nil
	}

	return true, "", nil
}

func (h *HumanEvalGoRunner) BuildRetryPrompt(problem formats.Problem, workDir string, validationError string) (string, error) {
	return fmt.Sprintf(`Your previous solution failed the tests. Error:
%s

Try again. Complete the function below.
Output ONLY the function body, no signature, no explanation, no markdown.

%s`, validationError, problem.Prompt), nil
}

func buildGoTestFile(problem formats.Problem, completion string) string {
	var sb strings.Builder

	sb.WriteString("package main\n\n")

	imports := mergeGoImports(problem.Import, problem.TestSetup)
	if imports != "" {
		sb.WriteString(imports)
		sb.WriteString("\n\n")
	}

	declaration := strings.TrimSpace(problem.Declaration)
	if declaration == "" {
		declaration = extractDeclaration(problem.Prompt)
	}
	sb.WriteString(declaration)
	sb.WriteString("\n")
	sb.WriteString(completion)
	sb.WriteString("\n\n")

	sb.WriteString(problem.Test)
	sb.WriteString("\n")

	return sb.String()
}

func mergeGoImports(codeImport, testSetup string) string {
	seen := map[string]bool{}
	var all []string

	for _, block := range []string{codeImport, testSetup} {
		re := regexp.MustCompile(`"([^"]+)"`)
		matches := re.FindAllStringSubmatch(block, -1)
		for _, m := range matches {
			pkg := m[1]
			if pkg == "testing" || seen[pkg] {
				continue
			}
			// skip package declaration lines
			if strings.HasPrefix(pkg, "package ") {
				continue
			}
			seen[pkg] = true
			all = append(all, pkg)
		}
	}

	// always need testing
	all = append([]string{"testing"}, all...)

	if len(all) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("import (\n")
	for _, pkg := range all {
		sb.WriteString(fmt.Sprintf("\t%q\n", pkg))
	}
	sb.WriteString(")")
	return sb.String()
}

func extractDeclaration(prompt string) string {
	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") {
			return line
		}
	}
	return ""
}

func cleanGoResponse(response string, problem formats.Problem) string {
	code := response

	codeBlockRe := regexp.MustCompile("(?s)```(?:go)?\\s*\n(.*?)\n```")
	if matches := codeBlockRe.FindStringSubmatch(code); len(matches) > 1 {
		code = matches[1]
	}

	thinkRe := regexp.MustCompile("(?s)<think>.*?</think>")
	code = thinkRe.ReplaceAllString(code, "")

	code = strings.TrimSpace(code)

	// Strip package declaration if present
	lines := strings.Split(code, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			continue
		}
		filtered = append(filtered, line)
	}
	code = strings.Join(filtered, "\n")
	code = strings.TrimSpace(code)

	// Strip import blocks if present
	importBlockRe := regexp.MustCompile(`(?s)import\s*\(.*?\)`)
	code = importBlockRe.ReplaceAllString(code, "")
	singleImportRe := regexp.MustCompile(`(?m)^import\s+"[^"]+"\s*$`)
	code = singleImportRe.ReplaceAllString(code, "")
	code = strings.TrimSpace(code)

	// If response includes the function signature, strip it to keep only body
	funcName := extractFuncName(problem.Declaration)
	if funcName != "" && strings.Contains(code, "func "+funcName) {
		lines := strings.Split(code, "\n")
		inBody := false
		braceDepth := 0
		var bodyLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !inBody && strings.Contains(trimmed, "func "+funcName) {
				inBody = true
				braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
				continue
			}
			if inBody {
				braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
				if braceDepth <= 0 {
					break
				}
				bodyLines = append(bodyLines, line)
			}
		}
		if len(bodyLines) > 0 {
			code = strings.Join(bodyLines, "\n")
		}
	}

	// Ensure proper indentation
	lines = strings.Split(code, "\n")
	var indented []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			indented = append(indented, "")
		} else if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "    ") {
			indented = append(indented, "\t"+line)
		} else {
			indented = append(indented, line)
		}
	}

	return "\n" + strings.Join(indented, "\n") + "\n}\n"
}

func extractFuncName(declaration string) string {
	re := regexp.MustCompile(`func\s+(\w+)`)
	if m := re.FindStringSubmatch(declaration); len(m) > 1 {
		return m[1]
	}
	return ""
}
