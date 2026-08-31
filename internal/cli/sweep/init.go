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
	"cache-type":       {"q4_0", "q8_0"},
	"batch":            {"2048", "4096"},
	"threads":          {"8", "12", "16"},
	"spec-draft-p-min": {"0.5", "0.75"},
	"spec-draft-n-max": {"4", "8"},
}

var paramDescriptions = map[string]string{
	"cache-type":       "KV cache quantization (applies to both K and V)",
	"batch":            "Prompt batch size (ubatch = batch/4)",
	"threads":          "CPU threads for prompt processing",
	"spec-draft-p-min": "Speculative decoding min probability threshold",
	"spec-draft-n-max": "Speculative decoding max draft tokens",
}

var toggleDescriptions = map[string]string{
	"np":        "Parallel sequences (on=-np 1, off=absent)",
	"load-mode": "Memory loading strategy (mlock, mmap, or off)",
}

var profileFieldDescriptions = map[string]string{
	"bin": "llama-server binary path",
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
		Profile:       profileName,
		Iterations:    5,
		ProfileFields: buildProfileFields(profileName),
		Parameters:    buildParameters(flagMap),
		Toggles:       buildToggles(flagMap),
	}

	doc := buildInitDocument(cfg)
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("failed to write sweep config: %w", err)
	}
	return enc.Close()
}

func buildInitDocument(cfg SweepConfig) *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	root := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = append(doc.Content, root)

	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "profile"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: cfg.Profile},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "iterations"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", cfg.Iterations), Tag: "!!int"},
	)

	if len(cfg.ProfileFields) > 0 {
		fieldsVal := &yaml.Node{Kind: yaml.MappingNode}
		for _, key := range sortedKeys(cfg.ProfileFields) {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
			if desc, ok := profileFieldDescriptions[key]; ok {
				keyNode.HeadComment = desc
			}
			fieldsVal.Content = append(fieldsVal.Content, keyNode, flowSequence(cfg.ProfileFields[key]))
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "profile_fields"},
			fieldsVal,
		)
	}

	if len(cfg.Parameters) > 0 {
		paramsVal := &yaml.Node{Kind: yaml.MappingNode}
		for _, key := range sortedKeys(cfg.Parameters) {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
			if desc, ok := paramDescriptions[key]; ok {
				keyNode.HeadComment = desc
			}
			paramsVal.Content = append(paramsVal.Content, keyNode, flowSequence(cfg.Parameters[key]))
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "parameters"},
			paramsVal,
		)
	}

	if len(cfg.Toggles) > 0 {
		togglesVal := &yaml.Node{Kind: yaml.MappingNode}
		for _, key := range sortedKeys(cfg.Toggles) {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
			if desc, ok := toggleDescriptions[key]; ok {
				keyNode.HeadComment = desc
			}
			togglesVal.Content = append(togglesVal.Content, keyNode, flowSequence(cfg.Toggles[key]))
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "toggles"},
			togglesVal,
		)
	}

	return doc
}

func flowSequence(values []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range values {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
	}
	return seq
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
		if aliases, isAlias := paramAliases[key]; isAlias {
			current := flagMap[aliases[0].Flag]
			alts := paramAlternatives[key]
			if current != "" {
				values := []string{current}
				for _, alt := range alts {
					if alt != current {
						values = append(values, alt)
					}
				}
				params[key] = values
			} else if len(alts) > 0 {
				params[key] = alts
			}
			continue
		}
		if _, isTarget := isAliasTarget(key); isTarget {
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

func isAliasTarget(param string) (string, bool) {
	for alias, targets := range paramAliases {
		for _, t := range targets {
			if t.Flag == param {
				return alias, true
			}
		}
	}
	return "", false
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
