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

func TestOutputImageIsValid_ValidCosiConfig(t *testing.T) {
	level := 15
	oi := OutputImage{
		Format: ImageFormatTypeCosi,
		Cosi: CosiConfig{
			Compression: CosiCompression{
				Level: &level,
			},
		},
	}
	err := oi.IsValid()
	assert.NoError(t, err)
}

func TestOutputImageIsValid_InvalidCosiConfig(t *testing.T) {
	level := 30
	oi := OutputImage{
		Format: ImageFormatTypeCosi,
		Cosi: CosiConfig{
			Compression: CosiCompression{
				Level: &level,
			},
		},
	}
	err := oi.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'cosi' field")
	assert.ErrorContains(t, err, "invalid 'compression' value")
	assert.ErrorContains(t, err, "invalid 'level' value")
}
