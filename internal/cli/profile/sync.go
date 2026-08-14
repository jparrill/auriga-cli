package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/huggingface"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type SyncResult struct {
	Name   string
	Status string // "ok", "skip", "fail", "warn"
	Detail string
}

func newProfileSyncCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download missing GGUF/mmproj for existing profiles",
		Long: `Sync profile files by downloading missing GGUFs and mmproj files.
Without --name, syncs all profiles. With --name, syncs only the specified profile.

Profiles with a repo field are auto-downloaded from HuggingFace.
Profiles without a repo show a warning with the missing filenames.

Examples:
  auriga profile sync
  auriga profile sync --name qwen3.6-vision`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileSync(name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Sync only this profile")

	return cmd
}

func runProfileSync(name string) error {
	var results []SyncResult

	if name != "" {
		existing := viper.GetStringMap(fmt.Sprintf("profiles.%s", name))
		if len(existing) == 0 {
			return fmt.Errorf("profile %q not found", name)
		}
		fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Syncing profile:"))
		results = []SyncResult{SyncProfile(name)}
	} else {
		results = SyncAllProfiles()
	}

	PrintSyncSummary(results)
	return nil
}

func SyncProfile(name string) SyncResult {
	repo := viper.GetString(fmt.Sprintf("profiles.%s.repo", name))
	model := viper.GetString(fmt.Sprintf("profiles.%s.model", name))
	mmproj := viper.GetString(fmt.Sprintf("profiles.%s.mmproj", name))

	if model == "" {
		detail := "profile has no model configured"
		ui.Fail(fmt.Sprintf("[%s] %s", name, detail))
		return SyncResult{Name: name, Status: "fail", Detail: detail}
	}

	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	modelPath := filepath.Join(ggufDir, model)
	modelExists := fileExists(modelPath)

	mmprojExists := true
	if mmproj != "" {
		mmprojPath := filepath.Join(mmprojDir, mmproj)
		mmprojExists = fileExists(mmprojPath)
	}

	if modelExists && mmprojExists {
		ui.Ok(fmt.Sprintf("[%s] All files present", name))
		return SyncResult{Name: name, Status: "skip", Detail: "all files present"}
	}

	if repo == "" {
		var missing []string
		if !modelExists {
			missing = append(missing, model)
		}
		if !mmprojExists {
			missing = append(missing, mmproj)
		}
		detail := fmt.Sprintf("no repo — manual download needed: %s", strings.Join(missing, ", "))
		ui.Warn(fmt.Sprintf("[%s] %s", name, detail))
		return SyncResult{Name: name, Status: "warn", Detail: detail}
	}

	ctx := context.Background()

	if !modelExists {
		url := huggingface.DownloadURL(repo, model)
		ui.Info(fmt.Sprintf("[%s] Downloading %s...", name, model))
		_, err := exec.Run(ctx, "wget", []string{"-c", url, "-O", modelPath}, exec.RunOpts{})
		if err != nil {
			detail := fmt.Sprintf("model download failed: %v", err)
			ui.Fail(fmt.Sprintf("[%s] %s", name, detail))
			return SyncResult{Name: name, Status: "fail", Detail: detail}
		}
		info, _ := os.Stat(modelPath)
		if info != nil {
			sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
			ui.Ok(fmt.Sprintf("[%s] Downloaded: %s (%.1f GB)", name, model, sizeGB))
		}
	}

	if !mmprojExists {
		originalName := repoFilename(mmproj, repo)
		url := huggingface.DownloadURL(repo, originalName)
		mmprojPath := filepath.Join(mmprojDir, mmproj)
		ui.Info(fmt.Sprintf("[%s] Downloading %s...", name, mmproj))
		_, err := exec.Run(ctx, "wget", []string{"-c", url, "-O", mmprojPath}, exec.RunOpts{})
		if err != nil {
			url = huggingface.DownloadURL(repo, mmproj)
			_, err = exec.Run(ctx, "wget", []string{"-c", url, "-O", mmprojPath}, exec.RunOpts{})
			if err != nil {
				detail := fmt.Sprintf("mmproj download failed: %v", err)
				ui.Fail(fmt.Sprintf("[%s] %s", name, detail))
				return SyncResult{Name: name, Status: "fail", Detail: detail}
			}
		}
		ui.Ok(fmt.Sprintf("[%s] Downloaded: %s", name, mmproj))
	}

	return SyncResult{Name: name, Status: "ok", Detail: "downloaded"}
}

func SyncAllProfiles() []SyncResult {
	profiles := viper.GetStringMap("profiles")
	if len(profiles) == 0 {
		ui.Warn("No profiles configured")
		return nil
	}

	fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Syncing profile files:"))

	var results []SyncResult
	for name := range profiles {
		results = append(results, SyncProfile(name))
	}
	return results
}

func PrintSyncSummary(results []SyncResult) {
	if len(results) == 0 {
		return
	}

	var ok, skip, warn, fail int
	for _, r := range results {
		switch r.Status {
		case "ok":
			ok++
		case "skip":
			skip++
		case "warn":
			warn++
		case "fail":
			fail++
		}
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("Summary: %d downloaded, %d already present, %d warnings, %d failed",
		ok, skip, warn, fail))
}

// repoFilename reverses the mmproj rename that setup/create apply.
// Profile stores "RepoBase-mmproj-f16.gguf" but the HF repo has "mmproj-f16.gguf".
func repoFilename(profileName, repo string) string {
	repoBase := filepath.Base(repo)
	repoBase = strings.TrimSuffix(repoBase, "-GGUF")
	if strings.HasPrefix(profileName, repoBase+"-") {
		return profileName[len(repoBase)+1:]
	}
	return profileName
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
