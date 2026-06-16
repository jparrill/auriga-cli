package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

type createOpts struct {
	Name      string
	Modelfile string
	GGUFFile  string
}

func newModelCreateCmd() *cobra.Command {
	opts := &createOpts{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an Ollama model from a Modelfile or GGUF",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelCreate(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Name, "name", "", "Model name for Ollama (required)")
	cmd.Flags().StringVar(&opts.Modelfile, "modelfile", "", "Path to Modelfile")
	cmd.Flags().StringVar(&opts.GGUFFile, "gguf", "", "Path to GGUF file (generates a simple Modelfile)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func runModelCreate(opts *createOpts) error {
	ctx := context.Background()

	var modelfilePath string

	if opts.Modelfile != "" {
		modelfilePath = opts.Modelfile
	} else if opts.GGUFFile != "" {
		ggufPath := opts.GGUFFile
		if !filepath.IsAbs(ggufPath) {
			ggufDir := config.ExpandHome(config.DefaultGGUFDir)
			ggufPath = filepath.Join(ggufDir, opts.GGUFFile)
		}

		if _, err := os.Stat(ggufPath); err != nil {
			return fmt.Errorf("GGUF not found: %s", ggufPath)
		}

		tmpFile, err := os.CreateTemp("", "auriga-modelfile-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmpFile.Name())

		content := fmt.Sprintf("FROM %s\n\nPARAMETER temperature 0.7\nPARAMETER top_p 0.95\nPARAMETER num_ctx 131072\n", ggufPath)
		tmpFile.WriteString(content)
		tmpFile.Close()
		modelfilePath = tmpFile.Name()
	} else {
		modelfilesDir := config.ExpandHome(config.DefaultModelfilesDir)
		modelfilePath = filepath.Join(modelfilesDir, opts.Name+".Modelfile")
		if _, err := os.Stat(modelfilePath); err != nil {
			return fmt.Errorf("no --modelfile or --gguf provided, and %s not found", modelfilePath)
		}
	}

	if _, err := os.Stat(modelfilePath); err != nil {
		return fmt.Errorf("Modelfile not found: %s", modelfilePath)
	}

	ui.Info(fmt.Sprintf("Creating Ollama model %q from %s", opts.Name, modelfilePath))

	err := ui.WithSpinner(fmt.Sprintf("Creating %s...", opts.Name), func() error {
		_, e := exec.RunCapture(ctx, "ollama", []string{"create", opts.Name, "-f", modelfilePath}, exec.RunOpts{})
		return e
	})
	if err != nil {
		return fmt.Errorf("ollama create failed: %w", err)
	}

	ui.Ok(fmt.Sprintf("Model %s created", opts.Name))
	return nil
}
