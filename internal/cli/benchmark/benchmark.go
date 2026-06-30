package benchmark

import (
	"github.com/spf13/cobra"
)

func NewBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "LLM web generation benchmark",
		Long: `Run and manage meta-benchmarks where LLMs generate Astro websites.

Examples:
  auriga benchmark list            # Show all results with pass/fail status
  auriga benchmark list --failed   # Only failed results`,
	}

	cmd.AddCommand(newBenchmarkListCmd())
	cmd.AddCommand(newBenchmarkRunCmd())
	cmd.AddCommand(newBenchmarkSuitesCmd())

	return cmd
}
