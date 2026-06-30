package benchmark

import (
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func init() {
	formats.Register("webgen", &WebgenRunner{})
}

type WebgenRunner struct{}

func (w *WebgenRunner) BuildPrompt(problem formats.Problem, suite formats.Suite) (string, error) {
	planFile := filepath.Join(suite.Dir, suite.PlanFile)
	sourceHTML := filepath.Join(suite.Dir, suite.SourceHTML)
	benchJSON := filepath.Join(suite.Dir, suite.BenchJSON)

	return BuildPrompt(planFile, sourceHTML, benchJSON)
}

func (w *WebgenRunner) ValidateResponse(response string, problem formats.Problem, workDir string) (bool, string, error) {
	parsed, _ := ParseFiles(response, workDir)
	if parsed == 0 {
		return false, "no_files", nil
	}

	violations := CheckSensitiveData(workDir)
	if len(violations) > 0 {
		desc := "sensitive_data:"
		for _, v := range violations {
			desc += " " + v.Description + " in " + v.FilePath + ";"
		}
		return false, desc, nil
	}

	buildOk, buildErr := ValidateBuild(workDir)
	if !buildOk {
		return false, "build_fail:" + buildErr, nil
	}

	return true, "", nil
}

func (w *WebgenRunner) BuildRetryPrompt(problem formats.Problem, workDir string, validationError string) (string, error) {
	if len(validationError) >= 8 && validationError[:8] == "no_files" {
		return BuildFormatRetryPrompt(""), nil
	}

	if len(validationError) >= 14 && validationError[:14] == "sensitive_data" {
		violations := CheckSensitiveData(workDir)
		return BuildSensitiveRetryPrompt(workDir, violations)
	}

	if len(validationError) >= 10 && validationError[:10] == "build_fail" {
		buildErr := validationError[11:]
		return BuildBuildRetryPrompt(workDir, buildErr)
	}

	return "", nil
}

func setupWorkDir(workDir string, attempt int) {
	if attempt == 1 {
		os.RemoveAll(workDir)
	}
	os.MkdirAll(workDir, 0755)
}
