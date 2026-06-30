package benchmark

import (
	"fmt"
	"strings"
	"time"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	_ "github.com/jparrill/auriga-cli/internal/benchmark/formats" // register formats
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type runOpts struct {
	Backend    string
	Models     string
	GenTimeout int
	Suite      string
	Host       string
}

func newBenchmarkRunCmd() *cobra.Command {
	opts := &runOpts{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run benchmark suite",
		Long: `Run a benchmark suite against one or more models.

Examples:
  auriga benchmark run                                          # Default suite, all models
  auriga benchmark run --suite humaneval                        # Specific suite
  auriga benchmark run --suite humaneval --models "gemma4:26b"  # Specific model
  auriga benchmark run --backend ollama --timeout 3600          # Ollama only, 1h timeout
  auriga benchmark run --host http://remote:8090                # Against remote server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Backend, "backend", "all", "Backend (ollama, llama-server, all)")
	cmd.Flags().StringVar(&opts.Models, "models", "", "Space-separated model list (overrides config)")
	cmd.Flags().IntVar(&opts.GenTimeout, "timeout", 0, "Generation timeout in seconds (default from config)")
	cmd.Flags().StringVar(&opts.Suite, "suite", "", "Benchmark suite to run (default: legacy webgen)")
	cmd.Flags().StringVar(&opts.Host, "host", "", "Override host URL (e.g., http://remote:8090)")

	return cmd
}

func runBenchmarkRun(opts *runOpts) error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
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

	// Apply host override
	if opts.Host != "" {
		viper.Set("ollama.host", opts.Host)
		viper.Set("llama_server.host", opts.Host)
	}

	suiteName := opts.Suite
	if suiteName == "" {
		suiteName = "(legacy webgen)"
	}

	params := []ui.OrderedParam{
		{Key: "Suite", Value: suiteName},
		{Key: "Backend", Value: opts.Backend},
		{Key: "Timeout", Value: fmt.Sprintf("%ds", genTimeout)},
		{Key: "Max retries", Value: fmt.Sprintf("%d", maxRetries)},
		{Key: "Results", Value: resultsDir},
	}
	if opts.Host != "" {
		params = append(params, ui.OrderedParam{Key: "Host", Value: opts.Host})
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

	cfg := bench.RunConfig{
		Backend:    opts.Backend,
		Models:     models,
		MaxRetries: maxRetries,
		MaxTokens:  maxTokens,
		GenTimeout: time.Duration(genTimeout) * time.Second,
		ResultsDir: resultsDir,
		Host:       opts.Host,
		SuiteName:  opts.Suite,
		// Legacy fields (used when no suite)
		PlanFile:   config.ExpandHome(viper.GetString("benchmark.plan_file")),
		SourceHTML: config.ExpandHome(viper.GetString("benchmark.source_html")),
		Benchmarks: config.ExpandHome(viper.GetString("benchmark.benchmarks_json")),
	}

	results, err := bench.RunAll(cfg)
	if err != nil {
		return err
	}

	bench.PrintSummary(results)
	return nil
}
