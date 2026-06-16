package profile

import (
	"fmt"

	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <profile-name>",
		Short: "Delete a profile from config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileDelete(args[0])
		},
	}
}

func runProfileDelete(name string) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if viper.GetString(profileKey+".model") == "" && viper.GetString(profileKey+".repo") == "" {
		return fmt.Errorf("profile %q not found", name)
	}

	if err := removeProfileFromConfig(name); err != nil {
		return fmt.Errorf("cannot update config: %w", err)
	}

	ui.Ok(fmt.Sprintf("Profile %q deleted from %s", name, configPath()))
	return nil
}
