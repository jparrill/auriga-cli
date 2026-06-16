package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/config"
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

	profiles := viper.GetStringMap("profiles")
	delete(profiles, name)
	viper.Set("profiles", profiles)

	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = config.ExpandHome("~/.config/auriga/config.yaml")
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}
	if err := viper.WriteConfigAs(cfgPath); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	ui.Ok(fmt.Sprintf("Profile %q deleted from %s", name, cfgPath))
	return nil
}
