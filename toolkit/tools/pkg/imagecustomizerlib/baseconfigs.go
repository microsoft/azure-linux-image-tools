package imagecustomizerlib

import (
	"context"
	"fmt"

	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

func ResolveBaseConfigs(ctx context.Context, cfg *imagecustomizerapi.Config, baseDir string, options ImageCustomizerOptions) (*ResolvedConfig, error) {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := BuildInheritanceChain(ctx, cfg, baseDir, visited, pathStack, options)
	if err != nil {
		return nil, err
	}

	resolved := NewResolvedConfig(configChain)

	return resolved, nil
}

func BuildInheritanceChain(ctx context.Context, cfg *imagecustomizerapi.Config, baseDir string, visited map[string]bool, pathStack []string, options ImageCustomizerOptions) ([]*imagecustomizerapi.Config, error) {
	if cfg.BaseConfigs != nil {
		for i, base := range cfg.BaseConfigs {
			err := base.IsValid()
			if err != nil {
				return nil, fmt.Errorf("invalid baseConfig item at index %d:\n%w", i, err)
			}
		}
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

		// Validate base config content
		err = ValidateConfig(ctx, baseDir, &baseCfg, false, options)
		if err != nil {
			return nil, fmt.Errorf("%w at %s:\n%v", ErrInvalidImageConfig, absPath, err)
		}

		// Recurse into base config
		subChain, err := BuildInheritanceChain(ctx, &baseCfg, filepath.Dir(absPath), visited, pathStack, options)
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
	for _, config := range chain {
		if config.Input != (imagecustomizerapi.Input{}) && config.Input.Image != (imagecustomizerapi.InputImage{}) && config.Input.Image.Path != "" {
			// .input.image.path
			target.InputImagePath = config.Input.Image.Path
		}

		if config.Output != (imagecustomizerapi.Output{}) {
			// .output.image.path
			if config.Output.Image != (imagecustomizerapi.OutputImage{}) && config.Output.Image.Path != "" {
				target.OutputImagePath = config.Output.Image.Path
			}
			// .output.image.format
			if config.Output.Image != (imagecustomizerapi.OutputImage{}) && config.Output.Image.Format != "" {
				target.OutputImageFormat = config.Output.Image.Format
			}
			// .output.image.artifacts.path
			if config.Output.Artifacts != nil {
				target.OutputArtifactsPath = config.Output.Artifacts.Path
			}
		}
	}
}

func resolveMergeFields(chain []*imagecustomizerapi.Config, target *ResolvedConfig) {
	for _, cfg := range chain {
		// .output.artifacts.items
		if cfg.Output != (imagecustomizerapi.Output{}) && cfg.Output.Artifacts != nil {
			target.OutputArtifactsItems = mergeOutputArtifactTypes(
				target.OutputArtifactsItems, cfg.Output.Artifacts.Items,
			)
		}
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
