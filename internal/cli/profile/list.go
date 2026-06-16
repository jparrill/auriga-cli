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

	maxName := len("PROFILE")
	maxModel := len("MODEL")
	maxRepo := len("REPO")

	type row struct{ name, repo, model, vision string }
	var rows []row

	for name := range profiles {
		repo := viper.GetString(fmt.Sprintf("profiles.%s.repo", name))
		model := viper.GetString(fmt.Sprintf("profiles.%s.model", name))
		mmproj := viper.GetString(fmt.Sprintf("profiles.%s.mmproj", name))
		vision := "no"
		if mmproj != "" {
			vision = "yes"
		}
		rows = append(rows, row{name, repo, model, vision})
		if len(name) > maxName {
			maxName = len(name)
		}
		if len(model) > maxModel {
			maxModel = len(model)
		}
		if len(repo) > maxRepo {
			maxRepo = len(repo)
		}
	}

	fmtStr := fmt.Sprintf("\n  %%-%ds  %%-%ds  %%-%ds  %%s\n", maxName, maxRepo, maxModel)
	fmt.Printf(fmtStr, "PROFILE", "REPO", "MODEL", "VISION")
	total := maxName + maxRepo + maxModel + 14
	fmt.Printf("  %s\n", repeatChar('─', total))

	for _, r := range rows {
		fmt.Printf(fmtStr, r.name, r.repo, r.model, r.vision)
	}
	fmt.Println()

	return nil
}

func repeatChar(c rune, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(c)
	}
	return string(b)
}
