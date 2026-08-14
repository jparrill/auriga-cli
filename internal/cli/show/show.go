package show

import (
	"github.com/spf13/cobra"
)

func NewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display configuration and connection details",
	}

	cmd.AddCommand(newShowConfigCmd())
	cmd.AddCommand(newShowPerfCmd())

	return cmd
}
