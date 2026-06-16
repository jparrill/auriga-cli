package benchmark

import (
	"fmt"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/benchmark"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type runOpts struct {
	Backend    string
	Models     string
	GenTimeout int
}

func newBenchmarkRunCmd() *cobra.Command {
	opts := &runOpts{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run LLM web generation benchmark",
		Long: `Run the meta-benchmark: give LLMs a project plan + source HTML and evaluate
the generated Astro website (format, sensitive data, build validation).

Examples:
  auriga benchmark run                                    # All configured models
  auriga benchmark run --backend ollama                   # Ollama only
  auriga benchmark run --backend llama-server             # llama-server only
  auriga benchmark run --models "gpt-oss:20b gemma4:26b"  # Specific models
  auriga benchmark run --timeout 3600                     # 1h timeout per model`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Backend, "backend", "all", "Backend (ollama, llama-server, all)")
	cmd.Flags().StringVar(&opts.Models, "models", "", "Space-separated model list (overrides config)")
	cmd.Flags().IntVar(&opts.GenTimeout, "timeout", 0, "Generation timeout in seconds (default from config)")

	return cmd
}

func runBenchmarkRun(opts *runOpts) error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
	planFile := config.ExpandHome(viper.GetString("benchmark.plan_file"))
	sourceHTML := config.ExpandHome(viper.GetString("benchmark.source_html"))
	benchmarksJSON := config.ExpandHome(viper.GetString("benchmark.benchmarks_json"))
	maxRetries := viper.GetInt("benchmark.max_retries")
	maxTokens := viper.GetInt("benchmark.max_tokens")
	genTimeout := viper.GetInt("benchmark.gen_timeout")

	if opts.GenTimeout > 0 {
		genTimeout = opts.GenTimeout
	}

	var models []string
	if opts.Models != "" {
		models = strings.Fields(opts.Models)
	}

	params := []ui.OrderedParam{
		{Key: "Backend", Value: opts.Backend},
		{Key: "Timeout", Value: fmt.Sprintf("%ds", genTimeout)},
		{Key: "Max retries", Value: fmt.Sprintf("%d", maxRetries)},
		{Key: "Results", Value: resultsDir},
	}
	if len(models) > 0 {
		params = append(params, ui.OrderedParam{Key: "Models", Value: strings.Join(models, ", ")})
	} else {
		params = append(params, ui.OrderedParam{Key: "Models", Value: "from config/env"})
	}

	confirmed, err := ui.ConfirmOperationOrdered("Run Benchmark", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	cfg := benchmark.RunConfig{
		Backend:    opts.Backend,
		Models:     models,
		MaxRetries: maxRetries,
		MaxTokens:  maxTokens,
		GenTimeout: time.Duration(genTimeout) * time.Second,
		ResultsDir: resultsDir,
		PlanFile:   planFile,
		SourceHTML: sourceHTML,
		Benchmarks: benchmarksJSON,
	}

	results, err := benchmark.RunAll(cfg)
	if err != nil {
		return err
	}

	benchmark.PrintSummary(results)
	return nil
}
