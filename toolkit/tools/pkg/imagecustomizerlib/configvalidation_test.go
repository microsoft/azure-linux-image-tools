// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestResolveCosiCompressionLevel_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil)

	assert.Equal(t, imagecustomizerapi.DefaultCosiCompressionLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_SingleConfig(t *testing.T) {
	configLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &configLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil)

	assert.Equal(t, configLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_CurrentConfigOverridesBase(t *testing.T) {
	baseConfigLevel := 9
	currConfigLevel := 22
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &baseConfigLevel,
							},
						},
					},
				},
			},
		},
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &currConfigLevel,
							},
						},
					},
				},
			},
		},
	}

	compression := resolveCosiCompressionLevel(configChain, nil)

	assert.Equal(t, currConfigLevel, compression)
}

func TestResolveCosiCompressionLevel_CLIOverridesConfig(t *testing.T) {
	configLevel := 22
	cliLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &configLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, &cliLevel)

	assert.Equal(t, cliLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_CLIOverridesBaseConfig(t *testing.T) {
	baseConfigLevel := 9
	currConfigLevel := 22
	cliLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &baseConfigLevel,
							},
						},
					},
				},
			},
		},
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &currConfigLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, &cliLevel)

	assert.Equal(t, cliLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_OnlyBaseConfigCompressionLevel(t *testing.T) {
	// Test the scenario described in the design doc:
	// "Inheriting compression without the preview feature in current config"
	level := 19
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &level,
							},
						},
					},
				},
			},
		},
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil)

	assert.Equal(t, 19, resolvedLevel)
}
