package profile

import (
	"fmt"
	"os"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/spf13/viper"
)

type ProfileConfig struct {
	Repo   string `yaml:"repo,omitempty"`
	Model  string `yaml:"model"`
	MMProj string `yaml:"mmproj,omitempty"`
}

func addProfileToConfig(name string, pc ProfileConfig) error {
	cfgPath := configPath()
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	profilesIdx := -1
	insertIdx := -1

	for i, line := range lines {
		if strings.TrimSpace(line) == "profiles:" {
			profilesIdx = i
		}
		result = append(result, line)
	}

	block := buildProfileBlock(name, pc)

	if profilesIdx == -1 {
		result = append(result, "", "profiles:")
		result = append(result, block...)
	} else {
		insertIdx = findProfilesEnd(lines, profilesIdx)
		// Walk back over trailing blank lines to insert right after last profile content
		for insertIdx > profilesIdx+1 && strings.TrimSpace(lines[insertIdx-1]) == "" {
			insertIdx--
		}
		tail := make([]string, len(result[insertIdx:]))
		copy(tail, result[insertIdx:])
		result = append(result[:insertIdx], block...)
		result = append(result, tail...)
	}

	return os.WriteFile(cfgPath, []byte(strings.Join(result, "\n")), 0644)
}

func removeProfileFromConfig(name string) error {
	cfgPath := configPath()
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	skip := false
	profileIndent := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == name+":" && skip == false {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if indent >= 2 {
				skip = true
				profileIndent = strings.Repeat(" ", indent)
				continue
			}
		}
		if skip {
			if strings.HasPrefix(line, profileIndent+"  ") || strings.TrimSpace(line) == "" {
				continue
			}
			skip = false
		}
		result = append(result, line)
	}

	return os.WriteFile(cfgPath, []byte(strings.Join(result, "\n")), 0644)
}

func buildProfileBlock(name string, pc ProfileConfig) []string {
	var lines []string
	lines = append(lines, fmt.Sprintf("  %s:", name))
	if pc.Repo != "" {
		lines = append(lines, fmt.Sprintf("    repo: %s", pc.Repo))
	}
	lines = append(lines, fmt.Sprintf("    model: %s", pc.Model))
	if pc.MMProj != "" {
		lines = append(lines, fmt.Sprintf("    mmproj: %s", pc.MMProj))
	}
	return lines
}

func findProfilesEnd(lines []string, profilesIdx int) int {
	for i := profilesIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			return i
		}
	}
	return len(lines)
}

func configPath() string {
	p := viper.ConfigFileUsed()
	if p == "" {
		p = config.ExpandHome("~/.config/auriga/config.yaml")
	}
	return p
}
