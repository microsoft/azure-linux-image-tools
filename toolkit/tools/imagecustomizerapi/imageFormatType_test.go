package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageFormatTypeIsValid_NoneIsValid(t *testing.T) {
	ft := ImageFormatTypeNone
	err := ft.IsValid()
	assert.NoError(t, err)
}

func TestImageFormatTypeIsValid_SupportedIsValid(t *testing.T) {
	for _, s := range SupportedImageFormatTypes() {
		ft := ImageFormatType(s)
		err := ft.IsValid()
		assert.NoError(t, err, "expected %s to be valid", s)
	}
}

func TestImageFormatTypeIsValid_UnsupportedIsInvalid(t *testing.T) {
	ft := ImageFormatType("xxx")
	err := ft.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid image format type (xxx)")
}
