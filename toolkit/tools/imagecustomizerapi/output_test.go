package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputIsValid(t *testing.T) {
	var output Output
	err := output.IsValid()
	assert.NoError(t, err)
}
