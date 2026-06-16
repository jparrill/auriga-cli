package benchmark

import (
	"github.com/spf13/cobra"
)

func NewBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "LLM web generation benchmark",
		Long:  "Run and manage meta-benchmarks where LLMs generate Astro websites.",
	}

	cmd.AddCommand(newBenchmarkListCmd())

	return cmd
}
