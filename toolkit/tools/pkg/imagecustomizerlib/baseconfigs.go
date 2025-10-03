package imagecustomizerlib

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

func loadBaseConfig(cfg *imagecustomizerapi.Config, baseDir string) (*ResolvedConfig, error) {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := BuildInheritanceChain(cfg, baseDir, visited, pathStack)
	if err != nil {
		return nil, err
	}

	resolved := NewResolvedConfig(configChain)

	data, err := yaml.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resolved config: %w", err)
	}
	fmt.Printf("ffffffffffffffffff:\n%s\n", string(data))

	return resolved, nil
}

func BuildInheritanceChain(cfg *imagecustomizerapi.Config, baseDir string, visited map[string]bool, pathStack []string) ([]*imagecustomizerapi.Config, error) {
	if err := cfg.BaseConfigs.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid baseConfigs:\n%w", err)
	}

	var chain []*imagecustomizerapi.Config

	for _, base := range cfg.BaseConfigs {
		basePath := filepath.Join(baseDir, base.Path)
		absPath, err := filepath.Abs(basePath)
		if err != nil {
			return nil, err
		}

		if visited[absPath] {
			return nil, fmt.Errorf("cycle detected in baseConfigs: %v -> %s", pathStack, absPath)
		}

		visited[absPath] = true
		pathStack = append(pathStack, absPath)

		// Load base file into struct
		var baseCfg imagecustomizerapi.Config
		if err := imagecustomizerapi.UnmarshalYamlFile(absPath, &baseCfg); err != nil {
			return nil, fmt.Errorf("failed to load base config %s: %w", absPath, err)
		}

		// Recurse into base config
		subChain, err := BuildInheritanceChain(&baseCfg, filepath.Dir(absPath), visited, pathStack)
		if err != nil {
			return nil, err
		}

		chain = append(chain, subChain...)
	}

	// Add the current config at the end
	chain = append(chain, cfg)

	return chain, nil
}

func resolveOverrideFields(chain []*imagecustomizerapi.Config, target *ResolvedConfig) {
	for _, cfg := range chain {
		// .input.image.path
		if val := strings.TrimSpace(cfg.Input.Image.Path); val != "" {
			target.InputImagePath = val
		}

		// .output.image.path
		if val := strings.TrimSpace(cfg.Output.Image.Path); val != "" {
			target.OutputImagePath = val
		}

		// .output.image.format
		if val := strings.TrimSpace(string(cfg.Output.Image.Format)); val != "" {
			target.OutputImageFormat = imagecustomizerapi.ImageFormatType(val)
		}

		// .output.image.artifacts.path
		if val := strings.TrimSpace(cfg.Output.Artifacts.Path); val != "" {
			target.OutputArtifactsPath = val
		}
	}
}

func resolveMergeFields(chain []*imagecustomizerapi.Config, target *ResolvedConfig) {
	for _, cfg := range chain {
		// .output.artifacts.items
		target.OutputArtifactsItems = mergeOutputArtifactTypes(
			target.OutputArtifactsItems, cfg.Output.Artifacts.Items,
		)
	}
}

func mergeOutputArtifactTypes(base, current []imagecustomizerapi.OutputArtifactsItemType) []imagecustomizerapi.OutputArtifactsItemType {
	seen := make(map[imagecustomizerapi.OutputArtifactsItemType]bool)
	var merged []imagecustomizerapi.OutputArtifactsItemType

	// Add base items first
	for _, item := range base {
		if !seen[item] {
			merged = append(merged, item)
			seen[item] = true
		}
	}

	// Add current items
	for _, item := range current {
		if !seen[item] {
			merged = append(merged, item)
			seen[item] = true
		}
	}

	return merged
}
