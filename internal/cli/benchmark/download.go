package benchmark

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

var knownSuites = map[string]struct {
	URL         string
	Description string
	Language    string
	Format      string
	Compressed  bool
}{
	"humaneval": {
		URL:         "https://github.com/openai/human-eval/raw/master/data/HumanEval.jsonl.gz",
		Description: "OpenAI HumanEval — 164 Python coding problems with unit tests",
		Language:    "python",
		Format:      "humaneval",
		Compressed:  true,
	},
}

func newBenchmarkDownloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "download <suite-name>",
		Short: "Download a benchmark suite",
		Long: `Download a known benchmark suite to ~/.config/auriga/suites/.

Available suites:
  humaneval    OpenAI HumanEval — 164 Python coding problems

Examples:
  auriga benchmark download humaneval`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkDownload(args[0])
		},
	}
}

func runBenchmarkDownload(name string) error {
	known, ok := knownSuites[name]
	if !ok {
		return fmt.Errorf("unknown suite %q. Available: humaneval", name)
	}

	suitesDir := bench.SuitesDir()
	suiteDir := filepath.Join(suitesDir, name)

	if _, err := os.Stat(filepath.Join(suiteDir, "suite.yaml")); err == nil {
		ui.Warn(fmt.Sprintf("Suite %q already exists at %s", name, suiteDir))
		return nil
	}

	params := []ui.OrderedParam{
		{Key: "Suite", Value: name},
		{Key: "Source", Value: known.URL},
		{Key: "Destination", Value: suiteDir},
	}

	confirmed, err := ui.ConfirmOperationOrdered("Download Suite", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	os.MkdirAll(suiteDir, 0755)

	// Download
	ui.Info(fmt.Sprintf("Downloading %s...", name))
	resp, err := http.Get(known.URL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if known.Compressed {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("cannot decompress: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	problemsPath := filepath.Join(suiteDir, "problems.jsonl")
	outFile, err := os.Create(problemsPath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, reader)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	ui.Ok(fmt.Sprintf("Downloaded %d bytes → %s", written, problemsPath))

	// Write suite.yaml
	suiteYaml := fmt.Sprintf(`name: %s
description: %s
language: %s
format: %s
source: %s
problems: problems.jsonl
runner: python3
`, name, known.Description, known.Language, known.Format, known.URL)

	yamlPath := filepath.Join(suiteDir, "suite.yaml")
	os.WriteFile(yamlPath, []byte(suiteYaml), 0644)
	ui.Ok(fmt.Sprintf("Created %s", yamlPath))

	ui.Ok(fmt.Sprintf("Suite %q ready", name))
	ui.Info(fmt.Sprintf("Run: auriga benchmark run --suite %s --models \"gemma4:26b\"", name))

	return nil
}
