package benchmark

import (
	"github.com/spf13/cobra"
)

func NewBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "LLM benchmark runner",
		Long: `Run and manage LLM benchmarks.

Examples:
  auriga benchmark list                     # List all benchmark runs
  auriga benchmark show                     # Show latest run details
  auriga benchmark show 2026-09-08_0956     # Show specific run
  auriga benchmark run --slot 1 --suite humaneval`,
	}

	cmd.AddCommand(newBenchmarkListCmd())
	cmd.AddCommand(newBenchmarkShowCmd())
	cmd.AddCommand(newBenchmarkRunCmd())
	cmd.AddCommand(newBenchmarkSuitesCmd())
	cmd.AddCommand(newBenchmarkDownloadCmd())
	cmd.AddCommand(newBenchmarkCompareCmd())

	return cmd
}
