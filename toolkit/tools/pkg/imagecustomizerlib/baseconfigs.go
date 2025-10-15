package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
)

type ConfigWithBasePath struct {
	*imagecustomizerapi.Config
	BaseConfigPath string
}

func resolveBaseConfigs(ctx context.Context, rc *ResolvedConfig) error {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := buildInheritanceChain(ctx, rc.Config, rc.BaseConfigPath, visited, pathStack)
	if err != nil {
		return err
	}

	merged := &ResolvedConfig{}

	resolveOverrideFields(configChain, merged)
	resolveMergeFields(configChain, merged)

	if merged.Config.Input != (imagecustomizerapi.Input{}) {
		rc.Config.Input = merged.Config.Input
	}
	if merged.Config.Output != (imagecustomizerapi.Output{}) {
		rc.Config.Output = merged.Config.Output
	}

	return nil
}

func buildInheritanceChain(ctx context.Context, cfg *imagecustomizerapi.Config, configFilePath string, visited map[string]bool,
	pathStack []string,
) ([]*ConfigWithBasePath, error) {
	var chain []*ConfigWithBasePath

	for _, base := range cfg.BaseConfigs {
		absPath := file.GetAbsPathWithBase(configFilePath, base.Path)

		if visited[absPath] {
			return nil, fmt.Errorf("cycle detected in baseConfigs: %v -> %s", pathStack, absPath)
		}

		visited[absPath] = true
		pathStack = append(pathStack, absPath)

		// Load base file into struct
		var baseCfg imagecustomizerapi.Config
		if err := imagecustomizerapi.UnmarshalYamlFile(absPath, &baseCfg); err != nil {
			return nil, fmt.Errorf("failed to load base config (%s):\n%w", absPath, err)
		}

		// Validate base config content
		if err := baseCfg.IsValid(); err != nil {
			return nil, fmt.Errorf("%w (%s):\n%w", ErrInvalidBaseConfigs, absPath, err)
		}

		// Recurse into base config
		subChain, err := buildInheritanceChain(ctx, &baseCfg, absPath, visited, pathStack)
		if err != nil {
			return nil, err
		}

		chain = append(chain, subChain...)
	}

	// Add the current config last
	chain = append(chain, &ConfigWithBasePath{
		Config:         cfg,
		BaseConfigPath: configFilePath,
	})

	return chain, nil
}

func resolveOverrideFields(chain []*ConfigWithBasePath, target *ResolvedConfig) {
	// Defensive initialization
	if target.Config == nil {
		target.Config = &imagecustomizerapi.Config{}
	}
	if target.Config.Input.Image == (imagecustomizerapi.InputImage{}) {
		target.Config.Input = imagecustomizerapi.Input{}
	}
	if target.Config.Output == (imagecustomizerapi.Output{}) {
		target.Config.Output = imagecustomizerapi.Output{}
	}

	for _, configWithBase := range chain {
		config := configWithBase.Config
		baseDir := configWithBase.BaseConfigPath

		// .input.image.path
		if config.Input.Image.Path != "" {
			absolutePath := file.GetAbsPathWithBase(baseDir, config.Input.Image.Path)
			target.Config.Input.Image.Path = absolutePath
		}

		// .output.image.path
		if config.Output.Image.Path != "" {
			absolutePath := file.GetAbsPathWithBase(baseDir, config.Output.Image.Path)
			target.Config.Output.Image.Path = absolutePath
		}

		// .output.image.format
		if config.Output.Image.Format != "" {
			target.Config.Output.Image.Format = config.Output.Image.Format
		}

		// .output.artifacts.path
		if config.Output.Artifacts != nil {
			if target.Config.Output.Artifacts == nil {
				target.Config.Output.Artifacts = &imagecustomizerapi.Artifacts{}
			}
			if config.Output.Artifacts.Path != "" {
				absolutePath := file.GetAbsPathWithBase(baseDir, config.Output.Artifacts.Path)
				target.Config.Output.Artifacts.Path = absolutePath
			}
		}
	}
}

func resolveMergeFields(chain []*ConfigWithBasePath, target *ResolvedConfig) {
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

func mergeOutputArtifactTypes(base, current []imagecustomizerapi.OutputArtifactsItemType,
) []imagecustomizerapi.OutputArtifactsItemType {
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
