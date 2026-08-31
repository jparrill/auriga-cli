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

	hasCustomBin := false
	for name := range profiles {
		if viper.GetString(fmt.Sprintf("profiles.%s.bin", name)) != "" {
			hasCustomBin = true
			break
		}
	}

	headers := []string{"PROFILE", "TYPE", "PORT", "SPEC", "REPO", "MODEL", "VISION"}
	if hasCustomBin {
		headers = append(headers, "BIN")
	}
	tbl := ui.NewTable("Profiles", headers...)

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

		row := []string{name, pType, fmt.Sprintf("%d", port), spec, repo, model, vision}
		if hasCustomBin {
			binOverride := viper.GetString(profileKey + ".bin")
			if binOverride != "" {
				row = append(row, binOverride)
			} else {
				row = append(row, "-")
			}
		}
		tbl.AddRow(row...)
	}

	tbl.Print()
	return nil
}
