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

func TestResolveIsoAdditionalFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolveIsoAdditionalFiles_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolveIsoAdditionalFiles_SingleConfig(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	content := "test content"
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "files/a.txt",
							Destination: "/a.txt",
							Permissions: &perms,
						},
						{
							Content:     &content,
							Destination: "/b.txt",
						},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/config/files/a.txt",
			Destination: "/a.txt",
			Permissions: &perms,
		},
		{
			Content:     &content,
			Destination: "/b.txt",
		},
	}, result.AdditionalFiles)
}

func TestResolveIsoAdditionalFiles_MultipleConfigs(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "base-files/base.txt",
							Destination: "/base.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "current-files/current.txt",
							Destination: "/current.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/base-files/base.txt",
			Destination: "/base.txt",
			Permissions: &perms,
		},
		{
			Source:      "/current/current-files/current.txt",
			Destination: "/current.txt",
			Permissions: &perms,
		},
	}, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_SingleConfig(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	content := "test content"
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "files/a.txt",
							Destination: "/a.txt",
							Permissions: &perms,
						},
						{
							Content:     &content,
							Destination: "/b.txt",
						},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/config/files/a.txt",
			Destination: "/a.txt",
			Permissions: &perms,
		},
		{
			Content:     &content,
			Destination: "/b.txt",
		},
	}, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_MultipleConfigs(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "base-files/base.txt",
							Destination: "/base.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "current-files/current.txt",
							Destination: "/current.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)

	// Base config's files should come first, then current config's files
	// Paths should be resolved relative to each config's base path
	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/base-files/base.txt",
			Destination: "/base.txt",
			Permissions: &perms,
		},
		{
			Source:      "/current/current-files/current.txt",
			Destination: "/current.txt",
			Permissions: &perms,
		},
	}, result.AdditionalFiles)
}

func TestResolveIsoKernelCommandLine_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0", "console=ttyS0"},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Equal(t, []string{"console=tty0", "console=ttyS0"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_MultipleConfigs(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.info", "rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)

	// Base config's args should come first, then current config's args are appended
	assert.Equal(t, []string{"console=tty0", "rd.info", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_EmptyArgsInMiddle(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{}, // Empty args in the middle config
					},
				},
			},
			BaseConfigPath: "/middle",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)

	// Should skip the empty config in the middle
	assert.Equal(t, []string{"console=tty0", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0", "console=ttyS0"},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Equal(t, []string{"console=tty0", "console=ttyS0"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_MultipleConfigs(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.info", "rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)

	// Base config's args should come first, then current config's args are appended
	assert.Equal(t, []string{"console=tty0", "rd.info", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_EmptyArgsInMiddle(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{}, // Empty args in the middle config
					},
				},
			},
			BaseConfigPath: "/middle",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)

	// Should skip the empty config in the middle
	assert.Equal(t, []string{"console=tty0", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoInitramfsType_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolveIsoInitramfsType_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolveIsoInitramfsType_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeBootstrap, result.InitramfsType)
}

func TestResolveIsoInitramfsType_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeFullOS, result.InitramfsType)
}

func TestResolveIsoInitramfsType_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeBootstrap, result.InitramfsType)
}

func TestResolvePxeInitramfsType_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolvePxeInitramfsType_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolvePxeInitramfsType_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeFullOS, result.InitramfsType)
}

func TestResolvePxeInitramfsType_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeBootstrap, result.InitramfsType)
}

func TestResolvePxeInitramfsType_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeFullOS, result.InitramfsType)
}

func TestResolveIsoKdumpBootFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolveIsoConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolveIsoConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_SingleConfig(t *testing.T) {
	kdumpType := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpType,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, &kdumpType, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_OverrideFromCurrent(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	kdumpNone := imagecustomizerapi.KdumpBootFilesTypeNone
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpNone,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, &kdumpNone, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_NilInCurrentUsesBase(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: nil, // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, &kdumpKeep, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_SingleConfig(t *testing.T) {
	kdumpType := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpType,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, &kdumpType, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_OverrideFromCurrent(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	kdumpNone := imagecustomizerapi.KdumpBootFilesTypeNone
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpNone,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, &kdumpNone, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_NilInCurrentUsesBase(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: nil, // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, &kdumpKeep, result.KdumpBootFiles)
}

func TestResolvePxeBootstrapBaseUrl_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://example.com/pxe/",
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://example.com/pxe/", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://base.example.com/pxe/",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://current.example.com/pxe/",
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://current.example.com/pxe/", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://base.example.com/pxe/",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://base.example.com/pxe/", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapFileUrl_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://example.com/pxe/image.iso",
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://example.com/pxe/image.iso", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://base.example.com/pxe/base.iso",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://current.example.com/pxe/current.iso",
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://current.example.com/pxe/current.iso", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://base.example.com/pxe/base.iso",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://base.example.com/pxe/base.iso", result.BootstrapFileUrl)
}
