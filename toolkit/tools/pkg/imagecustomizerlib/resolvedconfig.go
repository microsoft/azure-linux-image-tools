package imagecustomizerlib

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

// ResolvedConfig represents resolved fields and a chain of base configs.
type ResolvedConfig struct {
	InputImagePath       string
	OutputImagePath      string
	OutputImageFormat    imagecustomizerapi.ImageFormatType
	OutputArtifactsPath  string
	OutputArtifactsItems []imagecustomizerapi.OutputArtifactsItemType

	InheritanceChain []*imagecustomizerapi.Config
}

func NewResolvedConfig(chain []*imagecustomizerapi.Config) *ResolvedConfig {
	resolved := &ResolvedConfig{
		InheritanceChain: chain,
	}

	resolveOverrideFields(chain, resolved)
	resolveMergeFields(chain, resolved)

	return resolved
}
