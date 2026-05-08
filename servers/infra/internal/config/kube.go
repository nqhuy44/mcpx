package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// KubeConfig holds the minimal information parsed from ~/.kube/config.
type KubeConfig struct {
	CurrentContext string
	Contexts       []string // context names
}

// kubeConfigFile is the raw YAML structure we care about.
type kubeConfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Contexts       []struct {
		Name string `yaml:"name"`
	} `yaml:"contexts"`
}

// ParseKubeConfig parses ~/.kube/config (or $KUBECONFIG).
// Returns an empty KubeConfig (not error) if the file does not exist.
func ParseKubeConfig() (*KubeConfig, error) {
	path := os.Getenv("KUBECONFIG")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &KubeConfig{}, nil
		}
		path = filepath.Join(home, ".kube", "config")
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &KubeConfig{}, nil
	}
	if err != nil {
		return nil, err
	}

	var raw kubeConfigFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cfg := &KubeConfig{
		CurrentContext: raw.CurrentContext,
	}
	for _, c := range raw.Contexts {
		if c.Name != "" {
			cfg.Contexts = append(cfg.Contexts, c.Name)
		}
	}
	return cfg, nil
}
