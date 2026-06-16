package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
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

Wraps run-llm-benchmark.py with the correct environment from config.

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
	ctx := context.Background()

	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
	scriptDir := filepath.Dir(resultsDir)
	scriptPath := filepath.Join(scriptDir, "run-llm-benchmark.py")

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("benchmark script not found: %s\nExpected in parent of results_dir", scriptPath)
	}

	env := map[string]string{
		"BENCH_RESULTS_DIR":    resultsDir,
		"BENCH_MAX_TOKENS":     fmt.Sprintf("%d", viper.GetInt("benchmark.max_tokens")),
		"BENCH_MAX_RETRIES":    fmt.Sprintf("%d", viper.GetInt("benchmark.max_retries")),
		"OLLAMA_HOST":          viper.GetString("ollama.host"),
		"LLAMA_SERVER_HOST":    viper.GetString("llama_server.host"),
		"LLAMA_SERVER_BIN":     config.ExpandHome(viper.GetString("llama_server.bin")),
		"LLAMA_SERVER_GGUF_DIR": config.ExpandHome(viper.GetString("llama_server.gguf_dir")),
		"LLAMA_SERVER_QUANT":   viper.GetString("llama_server.quant"),
	}

	planFile := viper.GetString("benchmark.plan_file")
	if planFile != "" {
		env["BENCH_PLAN_FILE"] = config.ExpandHome(planFile)
	}
	sourceHTML := viper.GetString("benchmark.source_html")
	if sourceHTML != "" {
		env["BENCH_SOURCE_HTML"] = config.ExpandHome(sourceHTML)
	}

	timeout := viper.GetInt("benchmark.gen_timeout")
	if opts.GenTimeout > 0 {
		timeout = opts.GenTimeout
	}
	env["BENCH_GEN_TIMEOUT"] = fmt.Sprintf("%d", timeout)

	if opts.Models != "" {
		if opts.Backend == "llama-server" {
			env["LLAMA_SERVER_MODELS"] = opts.Models
			env["OLLAMA_MODELS"] = ""
		} else {
			env["OLLAMA_MODELS"] = opts.Models
			env["LLAMA_SERVER_MODELS"] = ""
		}
	}

	args := []string{scriptPath}
	if opts.Backend != "all" {
		args = append(args, "--backend", opts.Backend)
	}

	params := []ui.OrderedParam{
		{Key: "Script", Value: scriptPath},
		{Key: "Backend", Value: opts.Backend},
		{Key: "Timeout", Value: fmt.Sprintf("%ds", timeout)},
	}
	if opts.Models != "" {
		params = append(params, ui.OrderedParam{Key: "Models", Value: opts.Models})
	} else {
		params = append(params, ui.OrderedParam{Key: "Models", Value: "from config/env"})
	}

	confirmed, err := ui.ConfirmOperationOrdered("Run Benchmark", params, strings.Join(append([]string{"python3"}, args...), " "), false)
	if err != nil || !confirmed {
		return err
	}

	ui.Info("Starting benchmark...")
	return exec.RunStreaming(ctx, "python3", args, exec.RunOpts{
		Dir: scriptDir,
		Env: env,
	})
}
