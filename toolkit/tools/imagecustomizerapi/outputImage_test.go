package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputImageIsValid(t *testing.T) {
	var oi OutputImage
	err := oi.IsValid()
	assert.NoError(t, err)
}
