package sweep

import (
	"github.com/spf13/cobra"
)

func NewSweepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Automated parameter tuning for llama-server profiles",
		Long: `Sweep through parameter combinations to find optimal llama-server settings.

Generate a sweep config, validate it, then run the full sweep:

  auriga sweep init qwen3.8-27b-q4 > sweep.yaml
  auriga sweep validate --config sweep.yaml
  auriga sweep run --config sweep.yaml`,
	}

	cmd.AddCommand(newSweepInitCmd())
	cmd.AddCommand(newSweepValidateCmd())
	cmd.AddCommand(newSweepRunCmd())

	return cmd
}
