package profile

import (
	"fmt"

	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList()
		},
	}
}

func runProfileList() error {
	profiles := viper.GetStringMap("profiles")
	if len(profiles) == 0 {
		ui.Warn("No profiles configured. Create one with: auriga profile create <name> --repo <hf-repo>")
		return nil
	}

	tbl := ui.NewTable("Profiles", "PROFILE", "REPO", "MODEL", "VISION")

	for name := range profiles {
		repo := viper.GetString(fmt.Sprintf("profiles.%s.repo", name))
		model := viper.GetString(fmt.Sprintf("profiles.%s.model", name))
		mmproj := viper.GetString(fmt.Sprintf("profiles.%s.mmproj", name))
		vision := "no"
		if mmproj != "" {
			vision = ui.SuccessStyle.Render("yes")
		}
		tbl.AddRow(name, repo, model, vision)
	}

	tbl.Print()
	return nil
}
