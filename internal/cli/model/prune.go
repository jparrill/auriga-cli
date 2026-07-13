package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ollama"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

type pruneCandidate struct {
	Name    string
	Size    int64
	ModTime time.Time
	Backend string // "ollama", "gguf", or "mmproj"
	Path    string // filesystem path for gguf/mmproj; empty for ollama
}

func newModelPruneCmd() *cobra.Command {
	var backend string

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove unused models to free disk space",
		Long: `Interactively select models to delete across Ollama and llama-server backends.

Models are listed sorted by size (largest first) with their last-modified date.
Use --yes to skip confirmation, --dry-run to preview deletions.

Examples:
  auriga model prune                          # Interactive picker (all backends)
  auriga model prune --backend llama-server   # Only GGUFs
  auriga model prune --backend ollama         # Only Ollama models
  auriga model prune --yes                    # Delete all selected without confirmation`,
	}

	cmd.Flags().StringVar(&backend, "backend", "all", "Backend to prune (ollama, llama-server, all)")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runModelPrune(backend)
	}

	return cmd
}

func runModelPrune(backend string) error {
	var candidates []pruneCandidate

	if backend == "all" || backend == "ollama" {
		ollamaCandidates, err := collectOllamaCandidates()
		if err != nil {
			ui.Warn(fmt.Sprintf("Cannot list Ollama models: %v", err))
		} else {
			candidates = append(candidates, ollamaCandidates...)
		}
	}

	if backend == "all" || backend == "llama-server" {
		ggufCandidates := collectGGUFCandidates(llamaserver.GGUFDir(), "gguf")
		candidates = append(candidates, ggufCandidates...)

		mmprojCandidates := collectGGUFCandidates(llamaserver.MMProjDir(), "mmproj")
		candidates = append(candidates, mmprojCandidates...)
	}

	if len(candidates) == 0 {
		ui.Info("No models found to prune")
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Size > candidates[j].Size
	})

	tbl := ui.NewTable("Available models", "BACKEND", "MODEL", "SIZE", "MODIFIED")
	for _, c := range candidates {
		tbl.AddRow(c.Backend, c.Name, formatSize(c.Size), c.ModTime.Format("2006-01-02"))
	}
	tbl.Print()

	selected, err := pickCandidates(candidates)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		ui.Info("Nothing selected")
		return nil
	}

	var totalBytes int64
	for _, c := range selected {
		totalBytes += c.Size
	}

	confirmed, err := ui.ConfirmOperationOrdered(
		"Delete selected models",
		[]ui.OrderedParam{
			{Key: "Count", Value: fmt.Sprintf("%d models", len(selected))},
			{Key: "Space freed", Value: formatSize(totalBytes)},
		},
		"",
		false,
	)
	if err != nil || !confirmed {
		ui.Info("Cancelled")
		return nil
	}

	for _, c := range selected {
		if config.DryRun {
			ui.Info(fmt.Sprintf("[dry-run] Would delete: %s (%s)", c.Name, c.Backend))
			continue
		}

		switch c.Backend {
		case "ollama":
			if err := ollama.DeleteModel(c.Name); err != nil {
				ui.Fail(fmt.Sprintf("Failed to delete %s: %v", c.Name, err))
			} else {
				ui.Ok(fmt.Sprintf("Deleted Ollama model: %s (%s)", c.Name, formatSize(c.Size)))
			}
		case "gguf", "mmproj":
			if err := os.Remove(c.Path); err != nil {
				ui.Fail(fmt.Sprintf("Failed to delete %s: %v", c.Path, err))
			} else {
				ui.Ok(fmt.Sprintf("Deleted %s: %s (%s)", c.Backend, c.Name, formatSize(c.Size)))
			}
		}
	}

	return nil
}

func collectOllamaCandidates() ([]pruneCandidate, error) {
	models, err := ollama.ListModels()
	if err != nil {
		return nil, err
	}

	var candidates []pruneCandidate
	for _, m := range models {
		modTime, _ := time.Parse(time.RFC3339Nano, m.ModifiedAt)
		candidates = append(candidates, pruneCandidate{
			Name:    m.Name,
			Size:    m.Size,
			ModTime: modTime,
			Backend: "ollama",
		})
	}
	return candidates, nil
}

func collectGGUFCandidates(dir, backend string) []pruneCandidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var candidates []pruneCandidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".gguf" && backend == "gguf" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, pruneCandidate{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Backend: backend,
			Path:    filepath.Join(dir, e.Name()),
		})
	}
	return candidates
}

func pickCandidates(candidates []pruneCandidate) ([]pruneCandidate, error) {
	if config.Yes {
		return candidates, nil
	}

	options := make([]huh.Option[int], len(candidates))
	for i, c := range candidates {
		label := fmt.Sprintf("[%s] %s (%s, %s)",
			c.Backend, c.Name, formatSize(c.Size), c.ModTime.Format("2006-01-02"))
		options[i] = huh.NewOption(label, i)
	}

	var selectedIdx []int
	err := huh.NewMultiSelect[int]().
		Title("Select models to delete").
		Options(options...).
		Value(&selectedIdx).
		Run()
	if err != nil {
		return nil, err
	}

	selected := make([]pruneCandidate, len(selectedIdx))
	for i, idx := range selectedIdx {
		selected[i] = candidates[idx]
	}
	return selected, nil
}

func formatSize(bytes int64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	if gb >= 1.0 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	mb := float64(bytes) / (1024 * 1024)
	return fmt.Sprintf("%.0f MB", mb)
}
