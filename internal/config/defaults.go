package config

import (
	"os"
	"path/filepath"
)

// Overrideable at compile time via ldflags:
//
//	-X github.com/jparrill/auriga-cli/internal/config.DefaultGGUFDir=/custom/path
var (
	DefaultConfigPath     = "~/.config/auriga/config.yaml"
	DefaultGGUFDir        = "~/infra/ai/models/gguf"
	DefaultMMProjDir      = "~/infra/ai/models/mmproj"
	DefaultModelfilesDir  = "~/infra/ai/models/modelfiles"
	DefaultProfilesDir    = "~/infra/ai/profiles"
	DefaultResultsDir     = "~/Projects/auriga-lab/results"
	DefaultLlamaServerBin = "~/infra/bin/llama-server"
	DefaultPiBin          = "~/.npm-global/bin/pi"
	DefaultOllamaHost     = "http://localhost:11434"
	DefaultLlamaServerHost = "http://localhost:8090"
	DefaultLlamaServerPort = 8090
	DefaultQuant          = "Q4_K_M"
)

func ExpandHome(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
