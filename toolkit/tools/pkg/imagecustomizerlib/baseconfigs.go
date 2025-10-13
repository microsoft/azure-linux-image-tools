package imagecustomizerlib

import (
	"context"
	"fmt"

	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

func ResolveBaseConfigs(ctx context.Context, config *imagecustomizerapi.Config, baseConfigPath string, options ImageCustomizerOptions) (*ResolvedConfig, error) {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := BuildInheritanceChain(ctx, config, baseConfigPath, visited, pathStack)
	if err != nil {
		return nil, err
	}

	rc := &ResolvedConfig{
		BaseConfigPath: baseConfigPath,
		Config:         config,
		Options:        options,
	}

	resolveOverrideFields(configChain, rc)
	resolveMergeFields(configChain, rc)

	return rc, nil
}

func BuildInheritanceChain(ctx context.Context, cfg *imagecustomizerapi.Config, baseDir string, visited map[string]bool, pathStack []string) ([]*imagecustomizerapi.Config, error) {
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
		if err := baseCfg.IsValid(); err != nil {
			return nil, fmt.Errorf("%w at %s:\n%v", ErrInvalidImageConfig, absPath, err)
		}

		// Recurse into base config
		subChain, err := BuildInheritanceChain(ctx, &baseCfg, filepath.Dir(absPath), visited, pathStack)
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
	// Defensive initialization to avoid nil dereference
	if target.Config == nil {
		target.Config = &imagecustomizerapi.Config{}
	}
	if target.Config.Input.Image == (imagecustomizerapi.InputImage{}) {
		target.Config.Input = imagecustomizerapi.Input{}
	}
	if target.Config.Output == (imagecustomizerapi.Output{}) {
		target.Config.Output = imagecustomizerapi.Output{}
	}

	fmt.Println("ffffffffffffffffff", chain[0].Input.Image.Path, chain[1].Output.Image.Path)
	for _, config := range chain {
		// .input.image.path
		if config.Input.Image.Path != "" {
			target.Config.Input.Image.Path = config.Input.Image.Path
			target.InputImageFile = config.Input.Image.Path
		}

		// .output.image.path
		if config.Output.Image.Path != "" {
			target.Config.Output.Image.Path = config.Output.Image.Path
			target.OutputImageFile = config.Output.Image.Path
		}

		// .output.image.format
		if config.Output.Image.Format != "" {
			target.Config.Output.Image.Format = config.Output.Image.Format
			target.OutputImageFormat = config.Output.Image.Format
		}

		// .output.artifacts.path
		if config.Output.Artifacts != nil {
			if target.Config.Output.Artifacts == nil {
				target.Config.Output.Artifacts = &imagecustomizerapi.Artifacts{}
			}
			if config.Output.Artifacts.Path != "" {
				target.Config.Output.Artifacts.Path = config.Output.Artifacts.Path
			}
		}

	}
}

func resolveMergeFields(chain []*imagecustomizerapi.Config, target *ResolvedConfig) {
	// Defensive initialization
	if target.Config.Output == (imagecustomizerapi.Output{}) {
		target.Config.Output = imagecustomizerapi.Output{}
	}

	for _, config := range chain {
		if config.Output.Artifacts != nil {
			if target.Config.Output.Artifacts == nil {
				target.Config.Output.Artifacts = &imagecustomizerapi.Artifacts{}
			}
			target.Config.Output.Artifacts.Items = mergeOutputArtifactTypes(
				target.Config.Output.Artifacts.Items,
				config.Output.Artifacts.Items,
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
