package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

// ResolvedConfig represents a resolved config and chain of base configs.
type ResolvedConfig struct {
	Config *imagecustomizerapi.Config

	InheritanceChain []*imagecustomizerapi.Config
}

func LoadBaseConfig(cfg *imagecustomizerapi.Config, baseDir string) (*ResolvedConfig, error) {
	visited := make(map[string]bool)
	pathStack := []string{}

	configChain, err := buildInheritanceChain(cfg, baseDir, visited, pathStack)
	if err != nil {
		return nil, err
	}

	resolved := &ResolvedConfig{
		InheritanceChain: configChain,
		Config: &imagecustomizerapi.Config{
			Input: imagecustomizerapi.Input{
				Image: imagecustomizerapi.InputImage{},
			},
			Output: imagecustomizerapi.Output{
				Image: imagecustomizerapi.OutputImage{},
				Artifacts: &imagecustomizerapi.Artifacts{
					Items: []imagecustomizerapi.OutputArtifactsItemType{},
				},
			},
		},
	}

	resolved.Resolve()

	return resolved, nil
}

func buildInheritanceChain(cfg *imagecustomizerapi.Config, baseDir string, visited map[string]bool, pathStack []string) ([]*imagecustomizerapi.Config, error) {
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
		subChain, err := buildInheritanceChain(&baseCfg, filepath.Dir(absPath), visited, pathStack)
		if err != nil {
			return nil, err
		}

		chain = append(chain, subChain...)
	}

	// Add the current config at the end
	chain = append(chain, cfg)

	return chain, nil
}

func (r *ResolvedConfig) Resolve() {
	r.resolveOverrideFields()
	r.resolveMergeFields()
}

func (r *ResolvedConfig) resolveOverrideFields() {
	for _, cfg := range r.InheritanceChain {
		// .input.image.path
		if val := strings.TrimSpace(cfg.Input.Image.Path); val != "" {
			r.Config.Input.Image.Path = val
		}

		// .output.image.path
		if val := strings.TrimSpace(cfg.Output.Image.Path); val != "" {
			r.Config.Output.Image.Path = val
		}

		// .output.image.format
		if val := strings.TrimSpace(string(cfg.Output.Image.Format)); val != "" {
			r.Config.Output.Image.Format = imagecustomizerapi.ImageFormatType(val)
		}

		// .output.image.artifacts.path
		if val := strings.TrimSpace(cfg.Output.Artifacts.Path); val != "" {
			r.Config.Output.Artifacts.Path = val
		}
	}
}

func (r *ResolvedConfig) resolveMergeFields() {
	for _, cfg := range r.InheritanceChain {
		// .output.artifacts.items
		r.Config.Output.Artifacts.Items = mergeArtifacts(
			r.Config.Output.Artifacts.Items,
			cfg.Output.Artifacts.Items,
		)
	}
}

func mergeArtifacts(base, current []imagecustomizerapi.OutputArtifactsItemType) []imagecustomizerapi.OutputArtifactsItemType {
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
