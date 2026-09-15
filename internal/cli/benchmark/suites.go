package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
Also shows downloadable suites from the registry not yet installed.

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

	installed := make(map[string]bool)

	if len(suites) > 0 {
		tbl := ui.NewTable("Installed Suites", "NAME", "FORMAT", "LANGUAGE", "STATUS", "DESCRIPTION")
		for _, s := range suites {
			installed[s.Name] = true
			formatStatus := s.Format
			if _, err := formats.Get(s.Format); err != nil {
				formatStatus = s.Format + " (no runner)"
			}

			// Check if problems file exists
			status := ui.SuccessStyle.Render("ready")
			if s.Problems != "" {
				problemsPath := filepath.Join(s.Dir, s.Problems)
				if _, err := os.Stat(problemsPath); err != nil {
					status = ui.WarningStyle.Render("no data")
				}
			}

			tbl.AddRow(s.Name, formatStatus, s.Language, status, s.Description)
		}
		tbl.Print()
	}

	registry, err := bench.LoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	var downloadable []struct{ name, desc string }
	for name, info := range registry {
		if !installed[name] {
			downloadable = append(downloadable, struct{ name, desc string }{name, info.Description})
		}
	}
	sort.Slice(downloadable, func(i, j int) bool {
		return downloadable[i].name < downloadable[j].name
	})

	if len(downloadable) > 0 {
		tbl := ui.NewTable("Available for Download", "NAME", "DESCRIPTION", "COMMAND")
		for _, d := range downloadable {
			tbl.AddRow(d.name, d.desc, fmt.Sprintf("auriga benchmark download %s", d.name))
		}
		tbl.Print()
	}

	if len(suites) == 0 && len(downloadable) == 0 {
		ui.Warn(fmt.Sprintf("No suites found in %s", bench.SuitesDir()))
	}

	return nil
}
