package imagecustomizerlib_test

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

func TestResolveOverrideFields_OutputOverrides(t *testing.T) {
	base := &imagecustomizerapi.Config{
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path: "base-image.vhdx",
			},
			Artifacts: &imagecustomizerapi.Artifacts{
				Path: "./base-artifacts",
				Items: []imagecustomizerapi.OutputArtifactsItemType{
					imagecustomizerapi.OutputArtifactsItemUkis,
				},
			},
		},
	}

	current := &imagecustomizerapi.Config{
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path: "current-image.vhdx",
			},
			Artifacts: &imagecustomizerapi.Artifacts{
				Path: "./current-artifacts",
				Items: []imagecustomizerapi.OutputArtifactsItemType{
					imagecustomizerapi.OutputArtifactsItemShim,
				},
			},
		},
	}

	resolved := &imagecustomizerlib.ResolvedConfig{
		InheritanceChain: []*imagecustomizerapi.Config{base, current},
		Config: &imagecustomizerapi.Config{
			Output: imagecustomizerapi.Output{
				Artifacts: &imagecustomizerapi.Artifacts{},
				Image:     imagecustomizerapi.OutputImage{},
			},
		},
	}

	resolved.Resolve()

	if resolved.Config.Output.Image.Path != "current-image.vhdx" {
		t.Errorf("expected path = current-image.vhdx, got %s", resolved.Config.Output.Image.Path)
	}

	if resolved.Config.Output.Artifacts.Path != "./current-artifacts" {
		t.Errorf("expected artifacts path = ./current-artifacts, got %s", resolved.Config.Output.Artifacts.Path)
	}

	expectedItems := []imagecustomizerapi.OutputArtifactsItemType{
		imagecustomizerapi.OutputArtifactsItemUkis,
		imagecustomizerapi.OutputArtifactsItemShim,
	}
	got := resolved.Config.Output.Artifacts.Items
	if len(got) != len(expectedItems) {
		t.Errorf("expected output artifacts length = %d, got %d", len(expectedItems), len(got))
	}

	for _, item := range expectedItems {
		found := false
		for _, g := range got {
			if g == item {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected item %q not found in result: %v", item, got)
		}
	}
}
