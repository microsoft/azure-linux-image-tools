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

func buildConfigChain(ctx context.Context, rc *ResolvedConfig) ([]*ConfigWithBasePath, error) {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := buildConfigChainHelper(ctx, rc.Config, rc.BaseConfigPath, visited, pathStack)
	if err != nil {
		return nil, err
	}

	return configChain, nil
}

func buildConfigChainHelper(ctx context.Context, cfg *imagecustomizerapi.Config, configFilePath string, visited map[string]bool,
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
		subChain, err := buildConfigChainHelper(ctx, &baseCfg, absPath, visited, pathStack)
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

func resolveOutputArtifacts(configChain []*ConfigWithBasePath) *imagecustomizerapi.Artifacts {
	var artifacts *imagecustomizerapi.Artifacts

	for _, configWithBase := range configChain {
		if configWithBase.Config.Output.Artifacts != nil {
			if artifacts == nil {
				artifacts = &imagecustomizerapi.Artifacts{}
			}

			// Artifacts path from current config overrides previous one
			if configWithBase.Config.Output.Artifacts.Path != "" {
				artifacts.Path = file.GetAbsPathWithBase(
					configWithBase.BaseConfigPath,
					configWithBase.Config.Output.Artifacts.Path,
				)
			}

			// Append items
			artifacts.Items = mergeOutputArtifactTypes(
				artifacts.Items,
				configWithBase.Config.Output.Artifacts.Items,
			)
		}
	}

	return artifacts
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
