package sweep

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func newSweepInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <profile-name>",
		Short: "Generate a sweep config template for a profile",
		Long: `Generate a YAML template with tunable parameters for the given profile.
The template is pre-populated with current values and common alternatives.

Example:
  auriga sweep init qwen3.8-27b-q4 > sweep.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSweepInit(args[0])
		},
	}
}

var paramAlternatives = map[string][]string{
	"threads":          {"8", "12", "16"},
	"spec-draft-p-min": {"0.5", "0.75"},
	"spec-draft-n-max": {"4", "8"},
}

var linkedParamSet = map[string]bool{
	"cache-type-k": true, "cache-type-v": true,
	"batch-size": true, "ubatch-size": true,
}

var defaultCacheTypes = []string{"q4_0", "q8_0"}

type batchPreset struct {
	batch  string
	ubatch string
}

var defaultBatchPresets = []batchPreset{
	{"2048", "512"},
	{"4096", "1024"},
}

func runSweepInit(profileName string) error {
	profileKey := fmt.Sprintf("profiles.%s", profileName)
	model := viper.GetString(profileKey + ".model")
	if model == "" {
		return fmt.Errorf("profile %q not found — run: auriga profile list", profileName)
	}

	flags := viper.GetStringSlice(profileKey + ".flags")
	flagMap := parseFlagPairs(flags)

	cfg := SweepConfig{
		Profile:          profileName,
		Iterations:       5,
		ProfileFields:    buildProfileFields(profileName),
		Parameters:       buildParameters(flagMap),
		LinkedParameters: buildLinkedParameters(flagMap),
		Toggles:          buildToggles(flagMap),
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal sweep config: %w", err)
	}

	_, err = os.Stdout.Write(data)
	return err
}

func parseFlagPairs(flags []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(flags); i++ {
		flag := flags[i]
		if parts := strings.SplitN(flag, "=", 2); len(parts) == 2 && strings.HasPrefix(parts[0], "-") {
			key := strings.TrimPrefix(parts[0], "--")
			key = strings.TrimPrefix(key, "-")
			m[key] = parts[1]
			continue
		}
		key := strings.TrimPrefix(flag, "--")
		key = strings.TrimPrefix(key, "-")
		if strings.HasPrefix(flag, "-") && i+1 < len(flags) && !strings.HasPrefix(flags[i+1], "--") {
			m[key] = flags[i+1]
			i++
		}
	}
	return m
}

func buildProfileFields(profileName string) map[string][]string {
	fields := make(map[string][]string)

	binOverride := viper.GetString(fmt.Sprintf("profiles.%s.bin", profileName))
	globalBin := viper.GetString("llama_server.bin")

	if binOverride != "" {
		if globalBin != "" && globalBin != binOverride {
			fields["bin"] = []string{binOverride, globalBin}
		} else {
			fields["bin"] = []string{binOverride}
		}
	} else if globalBin != "" {
		fields["bin"] = []string{globalBin}
	}

	return fields
}

func buildParameters(flagMap map[string]string) map[string][]string {
	params := make(map[string][]string)

	for key := range knownParameters {
		if linkedParamSet[key] {
			continue
		}
		current, exists := flagMap[key]
		alts, hasAlts := paramAlternatives[key]

		if exists {
			values := []string{current}
			if hasAlts {
				for _, alt := range alts {
					if alt != current {
						values = append(values, alt)
					}
				}
			}
			params[key] = values
		} else if hasAlts {
			params[key] = alts
		}
	}

	return params
}

func buildLinkedParameters(flagMap map[string]string) map[string]map[string][]string {
	linked := make(map[string]map[string][]string)

	currentK := flagMap["cache-type-k"]
	if currentK == "" {
		currentK = "q4_0"
	}
	var cacheValues []string
	cacheValues = append(cacheValues, currentK)
	for _, alt := range defaultCacheTypes {
		if alt != currentK {
			cacheValues = append(cacheValues, alt)
		}
	}
	linked["cache-type"] = map[string][]string{
		"cache-type-k": cacheValues,
		"cache-type-v": cacheValues,
	}

	currentBatch := flagMap["batch-size"]
	currentUbatch := flagMap["ubatch-size"]
	var batchVals, ubatchVals []string
	if currentBatch != "" && currentUbatch != "" {
		batchVals = append(batchVals, currentBatch)
		ubatchVals = append(ubatchVals, currentUbatch)
	}
	for _, p := range defaultBatchPresets {
		if p.batch != currentBatch || p.ubatch != currentUbatch {
			batchVals = append(batchVals, p.batch)
			ubatchVals = append(ubatchVals, p.ubatch)
		}
	}
	linked["batch"] = map[string][]string{
		"batch-size":  batchVals,
		"ubatch-size": ubatchVals,
	}

	return linked
}

func buildToggles(flagMap map[string]string) map[string][]string {
	toggles := make(map[string][]string)

	if _, hasNP := flagMap["np"]; hasNP {
		toggles["np"] = []string{"on", "off"}
	}

	if val, hasLM := flagMap["load-mode"]; hasLM {
		toggles["load-mode"] = []string{val, "off"}
	}

	return toggles
}
