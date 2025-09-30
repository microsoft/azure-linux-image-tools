package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

// ResolvedConfig represents a chain of hierarchical configs along with resolved fields.
type ResolvedConfig struct {
	Input  *imagecustomizerapi.Input
	Output *imagecustomizerapi.Output

	// The raw inheritance chain: [base1, base2, ..., current]
	InheritanceChain []*imagecustomizerapi.Config
}

// LoadHierarchicalConfig loads a config with its inheritance chain and resolves override fields
func LoadHierarchicalConfig(configPath string) (*ResolvedConfig, error) {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := loadConfigRecursive(configPath, visited, pathStack)
	if err != nil {
		return nil, err
	}

	resolved := &ResolvedConfig{
		InheritanceChain: configChain,
	}

	// Optionally resolve fields like `.output.path`, `.iso.label`, etc.
	resolved.resolveOverrideFields()

	return resolved, nil
}

// loadConfigRecursive recursively loads base configs in DFS order.
func loadConfigRecursive(configPath string, visited map[string]bool, pathStack []string) ([]*imagecustomizerapi.Config, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path of %s: %w", configPath, err)
	}

	if visited[absPath] {
		return nil, fmt.Errorf("cyclic dependency detected: %v -> %s", pathStack, absPath)
	}

	visited[absPath] = true
	pathStack = append(pathStack, absPath)

	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(absPath, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s:\n%w", absPath, err)
	}

	// Recursively load base configs
	var chain []*imagecustomizerapi.Config
	for _, base := range config.BaseConfigs {
		basePath := filepath.Join(filepath.Dir(absPath), base.Path)
		baseChain, err := loadConfigRecursive(basePath, visited, pathStack)
		if err != nil {
			return nil, err
		}
		chain = append(chain, baseChain...)
	}

	// Add the current config last (after all its bases)
	chain = append(chain, &config)
	return chain, nil
}

// resolveOverrideFields resolves all fields that use the 'current overrides base' merge strategy.
func (r *ResolvedConfig) resolveOverrideFields() {
	// Iterate from base -> current
	for _, cfg := range r.InheritanceChain {
		// .output.artifacts.path
		if val := strings.TrimSpace(cfg.Output.Artifacts.Path); val != "" {
			r.Output.Artifacts.Path = val
		}
		// .output.image.path
		if val := strings.TrimSpace(cfg.Output.Image.Path); val != "" {
			r.Output.Image.Path = val
		}
		// .output.image.format
		if val := strings.TrimSpace(string(cfg.Output.Image.Format)); val != "" {
			r.Output.Image.Format = imagecustomizerapi.ImageFormatType(val)
		}
	}
}

// NewResolvedConfig takes a fully ordered inheritance chain (base first, current last)
// and resolves the final effective configuration for completely overridden fields.
func NewResolvedConfig(chain []*imagecustomizerapi.Config) (*ResolvedConfig, error) {
	if len(chain) == 0 {
		return nil, fmt.Errorf("config chain is empty")
	}

	// Apply complete override fields: take the LAST non-nil occurrence
	var resolvedInput *imagecustomizerapi.Input
	var resolvedOutput *imagecustomizerapi.Output

	for _, cfg := range chain {
		if cfg.Input == (imagecustomizerapi.Input{}) {
			resolvedInput = &cfg.Input
		}
		if cfg.Output == (imagecustomizerapi.Output{}) {
			resolvedOutput = &cfg.Output
		}
	}

	return &ResolvedConfig{
		Input:            resolvedInput,
		Output:           resolvedOutput,
		InheritanceChain: chain,
	}, nil
}
