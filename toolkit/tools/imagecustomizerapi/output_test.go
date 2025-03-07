package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputIsValid_EmptyIsValid(t *testing.T) {
	output := Output{}
	err := output.IsValid()
	assert.NoError(t, err)
}

func TestOutputIsValid_InvalidImageIsInvalid(t *testing.T) {
	output := Output{
		Image: OutputImage{
			Format: ImageFormatType("xxx"),
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'image' field")
}
