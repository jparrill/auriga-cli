package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type orphanedFile struct {
	Path string
	Dir  string
	Size int64
}

func newProfilePruneCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete orphaned model files not referenced by any profile",
		Long: `Scan GGUF and mmproj directories for files not referenced by any profile
and offer to delete them. Use --dry-run to preview without deleting.

Examples:
  auriga profile prune
  auriga profile prune --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfilePrune(dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show orphaned files without deleting")

	return cmd
}

func runProfilePrune(dryRun bool) error {
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	referenced := referencedFiles()

	var orphans []orphanedFile

	ggufOrphans, err := findOrphans(ggufDir, "gguf", referenced)
	if err != nil {
		ui.Warn(fmt.Sprintf("Cannot scan %s: %v", ggufDir, err))
	}
	orphans = append(orphans, ggufOrphans...)

	if mmprojDir != ggufDir {
		mmprojOrphans, err := findOrphans(mmprojDir, "mmproj", referenced)
		if err != nil {
			ui.Warn(fmt.Sprintf("Cannot scan %s: %v", mmprojDir, err))
		}
		orphans = append(orphans, mmprojOrphans...)
	}

	if len(orphans) == 0 {
		ui.Ok("No orphaned files found")
		return nil
	}

	sort.Slice(orphans, func(i, j int) bool {
		return orphans[i].Size > orphans[j].Size
	})

	var totalSize int64
	tbl := ui.NewTable("Orphaned files", "FILE", "DIR", "SIZE")
	for _, o := range orphans {
		tbl.AddRow(filepath.Base(o.Path), o.Dir, exec.FormatSize(o.Size))
		totalSize += o.Size
	}
	tbl.Print()
	ui.Info(fmt.Sprintf("Total: %s across %d files", exec.FormatSize(totalSize), len(orphans)))

	if dryRun {
		ui.Info("Dry run — no files deleted")
		return nil
	}

	confirmed, err := ui.ConfirmOperationOrdered("Delete orphaned files", []ui.OrderedParam{
		{Key: "Files", Value: fmt.Sprintf("%d", len(orphans))},
		{Key: "Space", Value: exec.FormatSize(totalSize)},
	}, "", false)
	if err != nil || !confirmed {
		return err
	}

	var deleted int
	var freedBytes int64
	for _, o := range orphans {
		if err := os.Remove(o.Path); err != nil {
			ui.Fail(fmt.Sprintf("Cannot delete %s: %v", filepath.Base(o.Path), err))
		} else {
			freedBytes += o.Size
			deleted++
		}
	}

	ui.Ok(fmt.Sprintf("Deleted %d files, freed %s", deleted, exec.FormatSize(freedBytes)))
	if deleted < len(orphans) {
		ui.Warn(fmt.Sprintf("%d files could not be deleted", len(orphans)-deleted))
	}
	return nil
}

func referencedFiles() map[string]bool {
	refs := make(map[string]bool)
	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		key := fmt.Sprintf("profiles.%s", name)
		for _, field := range []string{".model", ".mmproj", ".mtp_drafter", ".dflash"} {
			if v := viper.GetString(key + field); v != "" {
				refs[v] = true
			}
		}
	}
	return refs
}

func findOrphans(dir, label string, referenced map[string]bool) ([]orphanedFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var orphans []orphanedFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		if referenced[e.Name()] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		orphans = append(orphans, orphanedFile{
			Path: filepath.Join(dir, e.Name()),
			Dir:  label,
			Size: info.Size(),
		})
	}
	return orphans, nil
}
