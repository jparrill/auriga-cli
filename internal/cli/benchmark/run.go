package benchmark

import (
	"fmt"
	"time"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	_ "github.com/jparrill/auriga-cli/internal/benchmark/formats" // register formats
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type runOpts struct {
	Slot        int
	Suite       string
	GenTimeout  int
	Temperature float64
}

func newBenchmarkRunCmd() *cobra.Command {
	opts := &runOpts{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run benchmark suite against a slot",
		Long: `Run a benchmark suite against a running llama-server slot.

The slot must already have a model loaded. The model is auto-detected
from the running instance.

Examples:
  auriga benchmark run --slot 1                        # Default suite on slot 1
  auriga benchmark run --slot 2 --suite humaneval      # HumanEval on slot 2
  auriga benchmark run --slot 1 --timeout 3600         # 1h timeout`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkRun(opts)
		},
	}

	cmd.Flags().IntVar(&opts.Slot, "slot", 0, "Slot to benchmark (1 or 2, required)")
	cmd.Flags().StringVar(&opts.Suite, "suite", "", "Benchmark suite to run (default: legacy webgen)")
	cmd.Flags().IntVar(&opts.GenTimeout, "timeout", 0, "Generation timeout in seconds (default from config)")
	cmd.Flags().Float64Var(&opts.Temperature, "temperature", 0.3, "LLM sampling temperature (0.0 = deterministic)")
	cmd.MarkFlagRequired("slot")

	return cmd
}

func runBenchmarkRun(opts *runOpts) error {
	if opts.Slot != 1 && opts.Slot != 2 {
		return fmt.Errorf("--slot must be 1 or 2, got %d", opts.Slot)
	}

	port := llamaserver.SlotPort(opts.Slot)
	host := llamaserver.HostForPort(port)
	viper.Set("llama_server.host", host)

	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
	maxRetries := viper.GetInt("benchmark.max_retries")
	maxTokens := viper.GetInt("benchmark.max_tokens")
	genTimeout := viper.GetInt("benchmark.gen_timeout")

	if opts.GenTimeout > 0 {
		genTimeout = opts.GenTimeout
	}

	suiteName := opts.Suite
	if suiteName == "" {
		suiteName = "(legacy webgen)"
	}

	params := []ui.OrderedParam{
		{Key: "Slot", Value: fmt.Sprintf("%d (port %d)", opts.Slot, port)},
		{Key: "Suite", Value: suiteName},
		{Key: "Host", Value: host},
		{Key: "Timeout", Value: fmt.Sprintf("%ds", genTimeout)},
		{Key: "Max retries", Value: fmt.Sprintf("%d", maxRetries)},
		{Key: "Results", Value: resultsDir},
	}

	confirmed, err := ui.ConfirmOperationOrdered("Run Benchmark", params, "", config.Yes)
	if err != nil || !confirmed {
		return err
	}

	cfg := bench.RunConfig{
		MaxRetries:  maxRetries,
		MaxTokens:   maxTokens,
		GenTimeout:  time.Duration(genTimeout) * time.Second,
		ResultsDir:  resultsDir,
		Host:        host,
		Temperature: opts.Temperature,
		SuiteName:   opts.Suite,
		PlanFile:    config.ExpandHome(viper.GetString("benchmark.plan_file")),
		SourceHTML:  config.ExpandHome(viper.GetString("benchmark.source_html")),
		Benchmarks:  config.ExpandHome(viper.GetString("benchmark.benchmarks_json")),
	}

	results, err := bench.RunAll(cfg)
	if err != nil {
		return err
	}

	bench.PrintSummary(results)
	return nil
}
