package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputImageIsValidEmpty(t *testing.T) {
	var ii InputImage
	err := ii.IsValid()
	assert.NoError(t, err)
}

func TestInputImageIsValidPath(t *testing.T) {
	ii := InputImage{
		Path: "image.vhdx",
	}
	assert.NoError(t, ii.IsValid())
}

func TestInputImageIsValidOci(t *testing.T) {
	ii := InputImage{
		Oci: &OciImage{
			Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
		},
	}
	assert.NoError(t, ii.IsValid())
}

func TestInputImageIsValidBothOciAndPath(t *testing.T) {
	ii := InputImage{
		Path: "image.vhdx",
		Oci: &OciImage{
			Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
		},
	}
	err := ii.IsValid()
	assert.ErrorContains(t, err, "cannot specify both 'path' and 'oci'")
}
