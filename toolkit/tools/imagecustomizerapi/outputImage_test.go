package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputImageIsValid_EmptyIsValid(t *testing.T) {
	oi := OutputImage{}
	err := oi.IsValid()
	assert.NoError(t, err)
}

func TestOutputImageIsValid_AnyPathIsValid(t *testing.T) {
	oi := OutputImage{
		Path: "anything",
	}
	err := oi.IsValid()
	assert.NoError(t, err)
}

func TestOutputImageIsValid_InvalidFormatIsInvalid(t *testing.T) {
	oi := OutputImage{
		Format: ImageFormatType("xxx"),
	}
	err := oi.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'format' field")
}
