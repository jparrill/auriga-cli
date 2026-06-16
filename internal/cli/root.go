package cli

import (
	"strings"

	"github.com/jparrill/auriga-cli/internal/cli/benchmark"
	"github.com/jparrill/auriga-cli/internal/cli/fix"
	"github.com/jparrill/auriga-cli/internal/cli/model"
	"github.com/jparrill/auriga-cli/internal/cli/profile"
	"github.com/jparrill/auriga-cli/internal/cli/ps"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auriga",
		Short: "AI server management CLI for auriga",
		Long: `Manage LLM models, benchmarks, and development workflows on the auriga AI server.

Examples:
  auriga model list                          # List all Ollama + GGUF models
  auriga model list --backend ollama         # Ollama models only
  auriga model ensure                        # Download missing models
  auriga model create --name my-model --gguf Qwen3.6.gguf

  auriga profile create mymodel --repo unsloth/gemma-4-12b-it-GGUF --vision
  auriga profile list                        # List configured profiles
  auriga profile serve qwen3.6-vision        # Start llama-server with profile
  auriga profile stop                        # Stop llama-server, restart Ollama

  auriga benchmark list                      # Show all benchmark results
  auriga benchmark list --failed             # Only failed results

  auriga fix                                 # Interactive fix with Pi (fzf picker)
  auriga fix --failed                        # Only pick from failed results
  auriga fix --model gemma4                  # Jump to a specific model`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ui.InitLogger(config.Verbose)
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().BoolVarP(&config.Verbose, "verbose", "v", false, "Enable debug logging")
	cmd.PersistentFlags().BoolVar(&config.DryRun, "dry-run", false, "Print commands without executing")
	cmd.PersistentFlags().BoolVarP(&config.Yes, "yes", "y", false, "Skip confirmation prompts")

	var cfgFile string
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default ~/.config/auriga/config.yaml)")

	cobra.OnInitialize(func() { initViper(cfgFile) })

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(profile.NewProfileCmd())
	cmd.AddCommand(model.NewModelCmd())
	cmd.AddCommand(fix.NewFixCmd())
	cmd.AddCommand(benchmark.NewBenchmarkCmd())
	cmd.AddCommand(ps.NewPsCmd())

	return cmd
}

func initViper(cfgFile string) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(config.ExpandHome("~/.config/auriga"))
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Legacy env vars (compatible with Python scripts .envrc)
	viper.BindEnv("ollama.host", "OLLAMA_HOST")
	viper.BindEnv("ollama.models", "OLLAMA_MODELS")
	viper.BindEnv("llama_server.host", "LLAMA_SERVER_HOST")
	viper.BindEnv("llama_server.bin", "LLAMA_SERVER_BIN")
	viper.BindEnv("llama_server.gguf_dir", "LLAMA_SERVER_GGUF_DIR")
	viper.BindEnv("llama_server.quant", "LLAMA_SERVER_QUANT")
	viper.BindEnv("llama_server.models", "LLAMA_SERVER_MODELS")
	viper.BindEnv("benchmark.max_tokens", "BENCH_MAX_TOKENS")
	viper.BindEnv("benchmark.max_retries", "BENCH_MAX_RETRIES")
	viper.BindEnv("benchmark.gen_timeout", "BENCH_GEN_TIMEOUT")
	viper.BindEnv("benchmark.results_dir", "BENCH_RESULTS_DIR")
	viper.BindEnv("benchmark.plan_file", "BENCH_PLAN_FILE")
	viper.BindEnv("benchmark.source_html", "BENCH_SOURCE_HTML")
	viper.BindEnv("benchmark.benchmarks_json", "BENCH_BENCHMARKS_JSON")

	// Defaults
	viper.SetDefault("ollama.host", config.DefaultOllamaHost)
	viper.SetDefault("llama_server.host", config.DefaultLlamaServerHost)
	viper.SetDefault("llama_server.bin", config.DefaultLlamaServerBin)
	viper.SetDefault("llama_server.gguf_dir", config.DefaultGGUFDir)
	viper.SetDefault("llama_server.mmproj_dir", config.DefaultMMProjDir)
	viper.SetDefault("llama_server.quant", config.DefaultQuant)
	viper.SetDefault("benchmark.results_dir", config.DefaultResultsDir)
	viper.SetDefault("benchmark.max_tokens", 32768)
	viper.SetDefault("benchmark.max_retries", 5)
	viper.SetDefault("benchmark.gen_timeout", 900)
	viper.SetDefault("pi.bin", config.DefaultPiBin)

	viper.ReadInConfig()
}
