// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionGt(t *testing.T) {
	assert.True(t, Version{2}.Gt(Version{1}))
	assert.True(t, Version{2}.Gt(Version{1, 0}))
	assert.True(t, Version{2}.Gt(Version{1, 1}))
	assert.True(t, Version{2, 0}.Gt(Version{1}))
	assert.True(t, Version{2, 1}.Gt(Version{1}))
}

func TestVersionGe(t *testing.T) {
	assert.True(t, Version{2}.Ge(Version{1}))
	assert.True(t, Version{2}.Ge(Version{2}))
}

func TestVersionLt(t *testing.T) {
	assert.True(t, Version{1}.Lt(Version{2}))
}

func TestVersionLe(t *testing.T) {
	assert.True(t, Version{1}.Le(Version{2}))
	assert.True(t, Version{1}.Le(Version{1}))
}

func TestVersionEq(t *testing.T) {
	assert.True(t, Version{1}.Eq(Version{1}))
	assert.True(t, Version{1}.Eq(Version{1, 0}))
	assert.True(t, Version{1, 0}.Eq(Version{1}))
	assert.True(t, Version{1, 0}.Eq(Version{1, 0}))
}

func TestVersionString(t *testing.T) {
	assert.Equal(t, "", Version{}.String())
	assert.Equal(t, "1", Version{1}.String())
	assert.Equal(t, "1.2", Version{1, 2}.String())
	assert.Equal(t, "1.2.3", Version{1, 2, 3}.String())
}

func TestParseBasicVersionEmpty(t *testing.T) {
	v, err := ParseBasicVersion("")
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestParseBasicVersionOneNum(t *testing.T) {
	v, err := ParseBasicVersion("09")
	assert.NoError(t, err)
	assert.Equal(t, Version{9}, v)
}

func TestParseBasicVersionTrailingDot(t *testing.T) {
	v, err := ParseBasicVersion("1.")
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestParseBasicVersionTwoNum(t *testing.T) {
	v, err := ParseBasicVersion("2.3")
	assert.NoError(t, err)
	assert.Equal(t, Version{2, 3}, v)
}

func TestParseBasicVersionInvalidChar1(t *testing.T) {
	v, err := ParseBasicVersion("2.3a")
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestParseBasicVersionInvalidChar2(t *testing.T) {
	v, err := ParseBasicVersion("b")
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestParseBasicVersionThreeNum(t *testing.T) {
	v, err := ParseBasicVersion("4.5.6")
	assert.NoError(t, err)
	assert.Equal(t, Version{4, 5, 6}, v)
}
