package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
)

type ConfigWithBasePath struct {
	Config         *imagecustomizerapi.Config
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
