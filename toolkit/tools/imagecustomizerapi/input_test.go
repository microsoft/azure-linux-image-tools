package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputIsValid(t *testing.T) {
	var input Input
	err := input.IsValid()
	assert.NoError(t, err)
}
