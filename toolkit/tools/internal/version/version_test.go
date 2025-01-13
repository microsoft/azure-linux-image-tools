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
