package benchmark

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Suite struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Language    string   `yaml:"language"`
	Format      string   `yaml:"format"`
	Source      string   `yaml:"source,omitempty"`
	Problems    string   `yaml:"problems,omitempty"`
	PromptsDir  string   `yaml:"prompts_dir,omitempty"`
	Validation  []string `yaml:"validation,omitempty"`
	Levels      []Level  `yaml:"levels,omitempty"`
	Runner      string   `yaml:"runner,omitempty"`
	PlanFile    string   `yaml:"plan_file,omitempty"`
	SourceHTML  string   `yaml:"source_html,omitempty"`
	BenchJSON   string   `yaml:"benchmarks_json,omitempty"`
	Evaluation  []string `yaml:"evaluation,omitempty"`
	Dir         string   `yaml:"-"`
}

type Level struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Problem struct {
	TaskID     string   `json:"task_id"`
	Prompt     string   `json:"prompt"`
	Test       string   `json:"test,omitempty"`
	EntryPoint string   `json:"entry_point,omitempty"`
	Level      string   `json:"level,omitempty"`
	Eval       []string `json:"eval,omitempty"`
	TestCmd    string   `json:"test_cmd,omitempty"`
}

func SuitesDir() string {
	dir := viper.GetString("benchmark.suites_dir")
	if dir != "" {
		return config.ExpandHome(dir)
	}
	return config.ExpandHome("~/.config/auriga/suites")
}

func LoadSuite(name string) (*Suite, error) {
	dir := filepath.Join(SuitesDir(), name)
	yamlPath := filepath.Join(dir, "suite.yaml")

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("suite %q not found: %w", name, err)
	}

	var suite Suite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("invalid suite.yaml for %q: %w", name, err)
	}
	suite.Dir = dir

	return &suite, nil
}

func ListSuites() ([]Suite, error) {
	dir := SuitesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read suites dir %s: %w", dir, err)
	}

	var suites []Suite
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		suite, err := LoadSuite(e.Name())
		if err != nil {
			continue
		}
		suites = append(suites, *suite)
	}
	return suites, nil
}

func LoadProblems(suite *Suite) ([]Problem, error) {
	if suite.Problems == "" {
		return nil, nil
	}

	path := filepath.Join(suite.Dir, suite.Problems)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open problems file: %w", err)
	}
	defer f.Close()

	var problems []Problem
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var p Problem
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			continue
		}
		problems = append(problems, p)
	}

	return problems, scanner.Err()
}
