package benchmark

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	downloadURL         string
	downloadDescription string
	downloadLanguage    string
	downloadFormat      string
	downloadRunner      string
	downloadCompressed  bool
)

func newBenchmarkDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <suite-name>",
		Short: "Download a benchmark suite",
		Long: `Download a benchmark suite to ~/.config/auriga/suites/.

If the suite is in the registry, downloads from its registered URL.
If --url is provided, registers the suite and downloads it.

Examples:
  auriga benchmark download humaneval-go
  auriga benchmark download gsm8k --url "https://example.com/gsm8k.jsonl.gz" --description "Grade school math" --format humaneval --language go --compressed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkDownload(args[0])
		},
	}

	cmd.Flags().StringVar(&downloadURL, "url", "", "URL to download the suite JSONL from")
	cmd.Flags().StringVar(&downloadDescription, "description", "", "Suite description")
	cmd.Flags().StringVar(&downloadLanguage, "language", "", "Programming language (go, python, etc.)")
	cmd.Flags().StringVar(&downloadFormat, "format", "humaneval", "Problem format (humaneval, humaneval-go, quality, webgen)")
	cmd.Flags().StringVar(&downloadRunner, "runner", "", "Runner for execution (go, python3, etc.)")
	cmd.Flags().BoolVar(&downloadCompressed, "compressed", false, "URL points to gzip-compressed file")

	return cmd
}

func runBenchmarkDownload(name string) error {
	registry, err := bench.LoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	entry, inRegistry := registry[name]

	if downloadURL != "" {
		entry = bench.RegistryEntry{
			URL:         downloadURL,
			Description: downloadDescription,
			Language:    downloadLanguage,
			Format:      downloadFormat,
			Compressed:  downloadCompressed,
			Runner:      downloadRunner,
		}
		if entry.Runner == "" {
			entry.Runner = entry.Language
		}

		registry[name] = entry
		if err := bench.SaveRegistry(registry); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}
		ui.Ok(fmt.Sprintf("Registered suite %q in registry", name))
		inRegistry = true
	}

	if !inRegistry {
		available := make([]string, 0, len(registry))
		for k := range registry {
			available = append(available, k)
		}
		sort.Strings(available)
		return fmt.Errorf("unknown suite %q. Available: %s\nTo add a new suite, use: auriga benchmark download %s --url <URL>", name, strings.Join(available, ", "), name)
	}

	suitesDir := bench.SuitesDir()
	suiteDir := filepath.Join(suitesDir, name)

	if _, err := os.Stat(filepath.Join(suiteDir, "suite.yaml")); err == nil {
		ui.Warn(fmt.Sprintf("Suite %q already exists at %s", name, suiteDir))
		return nil
	}

	params := []ui.OrderedParam{
		{Key: "Suite", Value: name},
		{Key: "Source", Value: entry.URL},
		{Key: "Destination", Value: suiteDir},
	}

	confirmed, err := ui.ConfirmOperationOrdered("Download Suite", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	os.MkdirAll(suiteDir, 0755)

	ui.Info(fmt.Sprintf("Downloading %s...", name))
	resp, err := http.Get(entry.URL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if entry.Compressed {
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

	runner := entry.Runner
	if runner == "" {
		runner = entry.Language
	}

	suiteYaml := fmt.Sprintf(`name: %s
description: %s
language: %s
format: %s
source: %s
problems: problems.jsonl
runner: %s
`, name, entry.Description, entry.Language, entry.Format, entry.URL, runner)

	yamlPath := filepath.Join(suiteDir, "suite.yaml")
	os.WriteFile(yamlPath, []byte(suiteYaml), 0644)
	ui.Ok(fmt.Sprintf("Created %s", yamlPath))

	ui.Ok(fmt.Sprintf("Suite %q ready", name))
	ui.Info(fmt.Sprintf("Run: auriga benchmark run --suite %s --slot 1", name))

	return nil
}
