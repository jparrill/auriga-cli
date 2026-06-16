package profile

import (
	"github.com/spf13/cobra"
)

func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage llama-server profiles (model + vision)",
		Long: `Create, list, serve, and delete llama-server profiles.

A profile defines which GGUF model (and optional mmproj for vision) to load.
The repo, model, and mmproj are auto-resolved from HuggingFace.

Examples:
  auriga profile create gemma4-12b-vision --repo unsloth/gemma-4-12b-it-GGUF --vision
  auriga profile create qwen3.6 --repo unsloth/Qwen3.6-35B-A3B-GGUF
  auriga profile create custom --repo unsloth/Qwen3-30B-A3B-GGUF --model Qwen3-30B-A3B-Q4_K_M.gguf
  auriga profile list
  auriga profile serve gemma4-12b-vision
  auriga profile stop
  auriga profile delete gemma4-12b-vision`,
	}

	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileServeCmd())
	cmd.AddCommand(newProfileStopCmd())
	cmd.AddCommand(newProfileDeleteCmd())

	return cmd
}
