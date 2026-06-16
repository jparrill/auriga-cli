package model

import (
	"github.com/spf13/cobra"
)

func NewModelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage LLM models (Ollama + llama-server GGUFs)",
		Long: `Manage LLM models across Ollama and llama-server backends.

Examples:
  auriga model list                                      # All models
  auriga model list --backend llama-server               # Only GGUFs
  auriga model ensure --backend ollama                   # Pull missing Ollama models
  auriga model create --name qwen3.6-uncensored --gguf Qwen3.6-Uncensored.gguf`,
	}

	cmd.AddCommand(newModelListCmd())
	cmd.AddCommand(newModelEnsureCmd())
	cmd.AddCommand(newModelCreateCmd())

	return cmd
}
