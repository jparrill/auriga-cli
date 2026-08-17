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

	tbl := ui.NewTable("Profiles", "PROFILE", "TYPE", "PORT", "SPEC", "REPO", "MODEL", "VISION")

	for name := range profiles {
		profileKey := fmt.Sprintf("profiles.%s", name)
		repo := viper.GetString(profileKey + ".repo")
		model := viper.GetString(profileKey + ".model")
		mmproj := viper.GetString(profileKey + ".mmproj")
		flags := viper.GetStringSlice(profileKey + ".flags")
		vision := "no"
		if mmproj != "" {
			vision = ui.SuccessStyle.Render("yes")
		}
		spec := "-"
		if viper.GetString(profileKey+".dflash") != "" {
			spec = ui.SuccessStyle.Render("dflash")
		}
		if containsFlag(flags, "draft-mtp") || viper.GetString(profileKey+".mtp_drafter") != "" {
			spec = ui.SuccessStyle.Render("mtp")
		}
		pType := profileType(name)
		port := profilePort(name)
		tbl.AddRow(name, pType, fmt.Sprintf("%d", port), spec, repo, model, vision)
	}

	tbl.Print()
	return nil
}
