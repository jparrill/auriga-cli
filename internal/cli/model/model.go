package model

import (
	"github.com/spf13/cobra"
)

func NewModelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage LLM models (Ollama + llama-server GGUFs)",
	}

	cmd.AddCommand(newModelListCmd())
	cmd.AddCommand(newModelEnsureCmd())
	cmd.AddCommand(newModelCreateCmd())

	return cmd
}
