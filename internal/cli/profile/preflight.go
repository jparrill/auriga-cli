package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/spf13/viper"
)

type profilePreflight struct {
	Name           string
	ModelFile      string
	ModelPath      string
	MMProjFile     string
	MMProjPath     string
	DFlashFile     string
	MTPDrafterFile string
	GGUFDir        string
	Type           string
	Port           int
	Binary         string
	Flags          []string
	ContextSize    int
}

type vllmPreflight struct {
	Name        string
	ModelDir    string
	Port        int
	Binary      string
	Flags       []string
	ContextSize int
}

func resolveProfilePreflight(name string, ctxSize, slot int, missingCommand string) (profilePreflight, error) {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelFile := viper.GetString(profileKey + ".model")
	if modelFile == "" {
		return profilePreflight{}, fmt.Errorf("profile %q not found — run: auriga profile list", name)
	}

	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
	mmprojFile := viper.GetString(profileKey + ".mmproj")
	mmprojPath := ""
	if mmprojFile != "" {
		mmprojPath = filepath.Join(mmprojDir, name, mmprojFile)
		if _, err := os.Stat(mmprojPath); err != nil {
			legacyPath := filepath.Join(mmprojDir, mmprojFile)
			if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {
				mmprojPath = legacyPath
			} else {
				mmprojPath = ""
			}
		}
	}

	preflight := profilePreflight{
		Name:           name,
		ModelFile:      modelFile,
		ModelPath:      filepath.Join(ggufDir, modelFile),
		MMProjFile:     mmprojFile,
		MMProjPath:     mmprojPath,
		DFlashFile:     viper.GetString(profileKey + ".dflash"),
		MTPDrafterFile: viper.GetString(profileKey + ".mtp_drafter"),
		GGUFDir:        ggufDir,
		Type:           profileType(name),
		Port:           llamaserver.SlotPort(slot),
		Binary:         llamaserver.BinForProfile(name),
		Flags:          append([]string(nil), viper.GetStringSlice(profileKey+".flags")...),
		ContextSize:    ctxSize,
	}

	if _, err := os.Stat(preflight.ModelPath); err != nil {
		return profilePreflight{}, fmt.Errorf("model not found: %s\nRun: %s --profile %s", preflight.ModelPath, missingCommand, name)
	}
	if mmprojFile != "" && preflight.MMProjPath == "" {
		return profilePreflight{}, fmt.Errorf("mmproj not found: %s\nRun: %s --profile %s", filepath.Join(mmprojDir, name, mmprojFile), missingCommand, name)
	}
	if preflight.DFlashFile != "" {
		path := filepath.Join(ggufDir, preflight.DFlashFile)
		if _, err := os.Stat(path); err != nil {
			return profilePreflight{}, fmt.Errorf("dflash drafter not found: %s\nRun: auriga profile sync --name %s", path, name)
		}
	}
	if preflight.MTPDrafterFile != "" {
		path := filepath.Join(ggufDir, preflight.MTPDrafterFile)
		if _, err := os.Stat(path); err != nil {
			return profilePreflight{}, fmt.Errorf("mtp_drafter not found: %s\nRun: auriga profile sync --name %s", path, name)
		}
	}
	return preflight, nil
}

func validateProfileBinary(preflight profilePreflight) error {
	if _, err := os.Stat(preflight.Binary); err != nil {
		return fmt.Errorf("llama-server binary not found: %s", preflight.Binary)
	}
	return nil
}

func resolveVLLMPreflight(name string, ctxSize, slot int) (vllmPreflight, error) {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelDir := config.ExpandHome(viper.GetString(profileKey + ".model_dir"))
	if modelDir == "" {
		return vllmPreflight{}, fmt.Errorf("profile %q: model_dir required for vllm backend", name)
	}

	if fi, err := os.Stat(modelDir); err != nil || !fi.IsDir() {
		return vllmPreflight{}, fmt.Errorf("model directory not found: %s", modelDir)
	}

	bin := llamaserver.BinForProfile(name)
	if bin == "" || bin == llamaserver.Bin() {
		bin = config.ExpandHome(viper.GetString("vllm.bin"))
	}
	if bin == "" {
		bin = "vllm"
	}

	return vllmPreflight{
		Name:        name,
		ModelDir:    modelDir,
		Port:        llamaserver.SlotPort(slot),
		Binary:      bin,
		Flags:       append([]string(nil), viper.GetStringSlice(profileKey+".flags")...),
		ContextSize: ctxSize,
	}, nil
}

func profileFlagsWithVision(flags []string, mmprojFile string) []string {
	result := append([]string(nil), flags...)
	if mmprojFile != "" && !containsFlag(result, "--jinja") {
		result = append(result, "--jinja")
	}
	return result
}
