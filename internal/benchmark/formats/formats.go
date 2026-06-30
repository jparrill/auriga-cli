package formats

import (
	"fmt"

	"github.com/jparrill/auriga-cli/internal/benchmark"
)

type FormatRunner interface {
	BuildPrompt(problem benchmark.Problem, suite benchmark.Suite) (string, error)
	ValidateResponse(response string, problem benchmark.Problem, workDir string) (bool, string, error)
	BuildRetryPrompt(problem benchmark.Problem, workDir string, validationError string) (string, error)
}

var registry = map[string]FormatRunner{}

func Register(name string, runner FormatRunner) {
	registry[name] = runner
}

func Get(name string) (FormatRunner, error) {
	r, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown suite format %q (available: %v)", name, Available())
	}
	return r, nil
}

func Available() []string {
	var names []string
	for k := range registry {
		names = append(names, k)
	}
	return names
}
