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

func TestInputImageIsValidOciOk(t *testing.T) {
	ii := InputImage{
		Oci: &OciImage{
			Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
		},
	}
	assert.NoError(t, ii.IsValid())
}

func TestInputImageIsValidOciBad(t *testing.T) {
	ii := InputImage{
		Oci: &OciImage{
			Uri: "mcr.microsoft.com",
		},
	}
	err := ii.IsValid()
	assert.ErrorContains(t, err, "invalid 'oci' field")
	assert.ErrorContains(t, err, "invalid 'uri' field")
}

func TestInputImageIsValidAZLOk(t *testing.T) {
	ii := InputImage{
		AzureLinux: &AzureLinuxImage{
			Variant: "minimal-os",
			Version: "3.0",
		},
	}
	assert.NoError(t, ii.IsValid())
}

func TestInputImageIsValidAZLBad(t *testing.T) {
	ii := InputImage{
		AzureLinux: &AzureLinuxImage{
			Variant: "minimal-os",
			Version: "3.0.0.0",
		},
	}
	err := ii.IsValid()
	assert.ErrorContains(t, err, "invalid 'azureLinux' field")
	assert.ErrorContains(t, err, "invalid 'version' field")
}

func TestInputImageIsValidBothOciAndPath(t *testing.T) {
	ii := InputImage{
		Path: "image.vhdx",
		Oci: &OciImage{
			Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
		},
	}
	err := ii.IsValid()
	assert.ErrorContains(t, err, "must only specify one of 'path', 'oci', and 'azureLinux'")
}

func TestInputImageIsValidBothAZLAndPath(t *testing.T) {
	ii := InputImage{
		Path: "image.vhdx",
		AzureLinux: &AzureLinuxImage{
			Variant: "minimal-os",
			Version: "3.0",
		},
	}
	err := ii.IsValid()
	assert.ErrorContains(t, err, "must only specify one of 'path', 'oci', and 'azureLinux'")
}
