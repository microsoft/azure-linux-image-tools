package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputImageIsValid(t *testing.T) {
	var ii InputImage
	err := ii.IsValid()
	assert.NoError(t, err)
}
