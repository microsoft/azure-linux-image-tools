package imagecustomizerlib_test

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

func TestBaseConfigsInput(t *testing.T) {
	base := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "base-image-1.vhdx",
			},
		},
	}

	current := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "base-image-2.vhdx",
			},
		},
	}

	chain := []*imagecustomizerapi.Config{base, current}

	resolved := imagecustomizerlib.NewResolvedConfig(chain)

	if resolved.InputImagePath != "base-image-2.vhdx" {
		t.Errorf("expected input image path is base-image-2.vhdx, got %s", resolved.InputImagePath)
	}
}

func TestBaseConfigsOutput(t *testing.T) {
	base := &imagecustomizerapi.Config{
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path: "output-image-1.vhdx",
			},
			Artifacts: &imagecustomizerapi.Artifacts{
				Path: "./artifacts-1",
				Items: []imagecustomizerapi.OutputArtifactsItemType{
					imagecustomizerapi.OutputArtifactsItemUkis,
				},
			},
		},
	}

	current := &imagecustomizerapi.Config{
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path: "output-image-2.vhdx",
			},
			Artifacts: &imagecustomizerapi.Artifacts{
				Path: "./artifacts-2",
				Items: []imagecustomizerapi.OutputArtifactsItemType{
					imagecustomizerapi.OutputArtifactsItemShim,
				},
			},
		},
	}

	chain := []*imagecustomizerapi.Config{base, current}

	resolved := imagecustomizerlib.NewResolvedConfig(chain)

	if resolved.OutputImagePath != "output-image-2.vhdx" {
		t.Errorf("expected output path is output-image-2.vhdx, got %s", resolved.OutputImagePath)
	}

	if resolved.OutputArtifactsPath != "./artifacts-2" {
		t.Errorf("expected artifacts path is ./artifacts-2, got %s", resolved.OutputArtifactsPath)
	}

	expectedItems := []imagecustomizerapi.OutputArtifactsItemType{
		imagecustomizerapi.OutputArtifactsItemUkis,
		imagecustomizerapi.OutputArtifactsItemShim,
	}
	got := resolved.OutputArtifactsItems
	if len(got) != len(expectedItems) {
		t.Errorf("expected output artifacts length is %d, got %d", len(expectedItems), len(got))
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
			t.Errorf("expected artifact item %q not found in result: %v", item, got)
		}
	}
}
