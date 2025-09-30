package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"

	"gopkg.in/yaml.v3"
)

// LoadAndResolveConfig loads a config YAML file and resolves the inheritance chain,
// returning a ResolvedConfig for use during customization.
func LoadAndResolveConfig(configPath string) (*ResolvedConfig, error) {
	chain, err := buildInheritanceChain(configPath)
	if err != nil {
		return nil, err
	}

	return NewResolvedConfig(chain)
}

// buildInheritanceChain recursively loads baseConfigs, detects cycles, and returns
// the full inheritance chain in order: [base1, base2, ..., current]
func buildInheritanceChain(configPath string) ([]*imagecustomizerapi.Config, error) {
	visited := make(map[string]bool)
	var stack []string
	var chain []*imagecustomizerapi.Config

	var dfs func(string) error
	dfs = func(path string) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		if visited[absPath] {
			// Already visited — cycle check
			for _, p := range stack {
				if p == absPath {
					return fmt.Errorf("cyclic dependency detected: %v → %s", stack, absPath)
				}
			}
			return nil
		}

		stack = append(stack, absPath)
		defer func() { stack = stack[:len(stack)-1] }()
		visited[absPath] = true

		cfg, err := loadSingleConfig(absPath)
		if err != nil {
			return fmt.Errorf("failed to load config %s: %w", absPath, err)
		}

		// Visit base configs first
		for _, base := range cfg.BaseConfigs {
			if err := dfs(base.Path); err != nil {
				return err
			}
		}

		chain = append(chain, cfg)
		return nil
	}

	if err := dfs(configPath); err != nil {
		return nil, err
	}

	return chain, nil
}

// loadSingleConfig parses a single YAML file into imagecustomizerapi.Config
func loadSingleConfig(configPath string) (*imagecustomizerapi.Config, error) {
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config imagecustomizerapi.Config
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Validate individual config content
	if err := config.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid config content: %w", err)
	}

	return &config, nil
}
