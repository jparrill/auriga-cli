package benchmark

import (
	"fmt"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newBenchmarkSuitesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "suites",
		Short: "List available benchmark suites",
		Long: `List all benchmark suites installed in ~/.config/auriga/suites/.

Examples:
  auriga benchmark suites`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkSuites()
		},
	}
}

func runBenchmarkSuites() error {
	suites, err := bench.ListSuites()
	if err != nil {
		return err
	}

	if len(suites) == 0 {
		ui.Warn(fmt.Sprintf("No suites found in %s", bench.SuitesDir()))
		ui.Info("Download one with: auriga benchmark download humaneval")
		return nil
	}

	tbl := ui.NewTable("Benchmark Suites", "NAME", "FORMAT", "LANGUAGE", "DESCRIPTION")
	for _, s := range suites {
		formatStatus := s.Format
		if _, err := formats.Get(s.Format); err != nil {
			formatStatus = s.Format + " (not implemented)"
		}
		tbl.AddRow(s.Name, formatStatus, s.Language, s.Description)
	}
	tbl.Print()

	return nil
}
