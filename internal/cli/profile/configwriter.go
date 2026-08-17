package profile

import (
	"fmt"
	"os"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type ProfileConfig struct {
	Repo   string   `yaml:"repo,omitempty"`
	Model  string   `yaml:"model"`
	MMProj string   `yaml:"mmproj,omitempty"`
	DFlash string   `yaml:"dflash,omitempty"`
	Type   string   `yaml:"type,omitempty"`
	Port   int      `yaml:"port,omitempty"`
	Flags  []string `yaml:"flags,omitempty"`
}

func addProfileToConfig(name string, pc ProfileConfig) error {
	doc, err := readConfigDoc()
	if err != nil {
		return err
	}

	root := doc.Content[0] // mapping node

	profilesNode := findMappingKey(root, "profiles")
	if profilesNode == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "profiles"},
			&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
		)
		profilesNode = root.Content[len(root.Content)-1]
	}

	profileValueNode := buildProfileNode(pc)
	profilesNode.Content = append(profilesNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		profileValueNode,
	)

	return writeConfigDoc(doc)
}

func removeProfileFromConfig(name string) error {
	doc, err := readConfigDoc()
	if err != nil {
		return err
	}

	root := doc.Content[0]
	profilesNode := findMappingKey(root, "profiles")
	if profilesNode == nil {
		return nil
	}

	var filtered []*yaml.Node
	for i := 0; i+1 < len(profilesNode.Content); i += 2 {
		if profilesNode.Content[i].Value != name {
			filtered = append(filtered, profilesNode.Content[i], profilesNode.Content[i+1])
		}
	}
	profilesNode.Content = filtered

	return writeConfigDoc(doc)
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
	if pc.DFlash != "" {
		lines = append(lines, fmt.Sprintf("    dflash: %s", pc.DFlash))
	}
	if pc.Type != "" {
		lines = append(lines, fmt.Sprintf("    type: %s", pc.Type))
	}
	if pc.Port > 0 {
		lines = append(lines, fmt.Sprintf("    port: %d", pc.Port))
	}
	if len(pc.Flags) > 0 {
		var flagNodes string
		for i, f := range pc.Flags {
			if i > 0 {
				flagNodes += ", "
			}
			flagNodes += f
		}
		lines = append(lines, fmt.Sprintf("    flags: [%s]", flagNodes))
	}
	return lines
}

func buildProfileNode(pc ProfileConfig) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	if pc.Repo != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "repo"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: pc.Repo},
		)
	}

	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "model"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: pc.Model},
	)

	if pc.MMProj != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "mmproj"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: pc.MMProj},
		)
	}

	if pc.DFlash != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "dflash"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: pc.DFlash},
		)
	}

	if pc.Type != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: pc.Type},
		)
	}

	if pc.Port > 0 {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "port"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", pc.Port)},
		)
	}

	if len(pc.Flags) > 0 {
		flagSeq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, f := range pc.Flags {
			flagSeq.Content = append(flagSeq.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: f},
			)
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "flags"},
			flagSeq,
		)
	}

	return node
}

func findMappingKey(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func readConfigDoc() (*yaml.Node, error) {
	cfgPath := configPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	if len(doc.Content) == 0 {
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
	}

	return &doc, nil
}

func writeConfigDoc(doc *yaml.Node) error {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	return os.WriteFile(configPath(), data, 0644)
}

func configPath() string {
	p := viper.ConfigFileUsed()
	if p == "" {
		p = config.ExpandHome("~/.config/auriga/config.yaml")
	}
	return p
}
