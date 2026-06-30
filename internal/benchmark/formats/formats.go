package formats

import "fmt"

type Problem struct {
	TaskID     string   `json:"task_id"`
	Prompt     string   `json:"prompt"`
	Test       string   `json:"test,omitempty"`
	EntryPoint string   `json:"entry_point,omitempty"`
	Level      string   `json:"level,omitempty"`
	Eval       []string `json:"eval,omitempty"`
	TestCmd    string   `json:"test_cmd,omitempty"`
}

type Suite struct {
	Name       string
	Format     string
	Language   string
	Dir        string
	PlanFile   string
	SourceHTML string
	BenchJSON  string
}

type FormatRunner interface {
	BuildPrompt(problem Problem, suite Suite) (string, error)
	ValidateResponse(response string, problem Problem, workDir string) (bool, string, error)
	BuildRetryPrompt(problem Problem, workDir string, validationError string) (string, error)
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
