// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestResolveCosiCompression_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	compression := resolveCosiCompression(configChain, 0)

	// When config chain is empty and no CLI args, zero values are used
	assert.Equal(t, 0, compression.Level)
}

func TestResolveCosiCompression_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: &imagecustomizerapi.CosiConfig{
							Compression: &imagecustomizerapi.CosiCompression{
								Level: 15,
							},
						},
					},
				},
			},
		},
	}

	compression := resolveCosiCompression(configChain, 0)

	assert.Equal(t, 15, compression.Level)
}

func TestResolveCosiCompression_MultipleConfigs_LastWins(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: &imagecustomizerapi.CosiConfig{
							Compression: &imagecustomizerapi.CosiCompression{
								Level: 9,
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
						Cosi: &imagecustomizerapi.CosiConfig{
							Compression: &imagecustomizerapi.CosiCompression{
								Level: 22,
							},
						},
					},
				},
			},
		},
	}

	compression := resolveCosiCompression(configChain, 0)

	assert.Equal(t, 22, compression.Level)
}

func TestResolveCosiCompression_CLIOverridesConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: &imagecustomizerapi.CosiConfig{
							Compression: &imagecustomizerapi.CosiCompression{
								Level: 9,
							},
						},
					},
				},
			},
		},
	}

	compression := resolveCosiCompression(configChain, 15)

	assert.Equal(t, 15, compression.Level)
}

func TestResolveCosiCompression_NilCosiConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: nil,
					},
				},
			},
		},
	}

	compression := resolveCosiCompression(configChain, 0)

	assert.Equal(t, 0, compression.Level)
}

func TestResolveCosiCompression_NilCompression(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: &imagecustomizerapi.CosiConfig{
							Compression: nil,
						},
					},
				},
			},
		},
	}

	compression := resolveCosiCompression(configChain, 0)

	assert.Equal(t, 0, compression.Level)
}

func TestResolveCosiCompression_BaseConfigWithCurrentNoCompression(t *testing.T) {
	// Test the scenario described in the design doc:
	// "Inheriting compression without the preview feature in current config"
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: &imagecustomizerapi.CosiConfig{
							Compression: &imagecustomizerapi.CosiCompression{
								Level: 19,
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

	compression := resolveCosiCompression(configChain, 0)

	assert.Equal(t, 19, compression.Level)
}
