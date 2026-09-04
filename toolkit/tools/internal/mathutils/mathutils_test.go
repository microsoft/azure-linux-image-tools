package mathutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundUp0(t *testing.T) {
	assert.Equal(t, uint32(0), RoundUp(uint32(0), 32))
}

func TestRoundUp1(t *testing.T) {
	assert.Equal(t, uint32(32), RoundUp(uint32(1), 32))
}

func TestRoundUp1Unit(t *testing.T) {
	assert.Equal(t, uint32(32), RoundUp(uint32(32), 32))
}

func TestRoundDown0(t *testing.T) {
	assert.Equal(t, uint32(0), RoundDown(uint32(0), 32))
}

func TestRoundDown1(t *testing.T) {
	assert.Equal(t, uint32(0), RoundDown(uint32(1), 32))
}

func TestRoundDown1Unit(t *testing.T) {
	assert.Equal(t, uint32(32), RoundDown(uint32(32), 32))
}

func TestRoundDown1AndHalfUnit(t *testing.T) {
	assert.Equal(t, uint32(32), RoundDown(uint32(48), 32))
}

func TestDivRoundUp0(t *testing.T) {
	assert.Equal(t, uint64(0), DivRoundUp(uint64(0), 32))
}

func TestDivRoundUp1(t *testing.T) {
	assert.Equal(t, uint64(1), DivRoundUp(uint64(1), 32))
}

func TestDivRoundUp1Unit(t *testing.T) {
	assert.Equal(t, uint64(1), DivRoundUp(uint64(32), 32))
}
