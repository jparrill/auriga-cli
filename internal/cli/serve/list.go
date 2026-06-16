package serve

import (
	"fmt"

	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newServeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available serve profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServeList()
		},
	}
}

func runServeList() error {
	profiles := viper.GetStringMap("profiles")
	if len(profiles) == 0 {
		ui.Warn("No profiles configured. Add profiles to your config.yaml.")
		return nil
	}

	fmt.Printf("\n  %-30s %-40s %-10s\n", "PROFILE", "MODEL", "VISION")
	fmt.Printf("  %s\n", "────────────────────────────────────────────────────────────────────────────────")

	for name := range profiles {
		model := viper.GetString(fmt.Sprintf("profiles.%s.model", name))
		mmproj := viper.GetString(fmt.Sprintf("profiles.%s.mmproj", name))
		vision := "no"
		if mmproj != "" {
			vision = "yes"
		}
		fmt.Printf("  %-30s %-40s %-10s\n", name, model, vision)
	}
	fmt.Println()

	return nil
}
