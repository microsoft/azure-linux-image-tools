package cosiapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The osPackages field must never be dropped from the COSI metadata, and an
// empty (non-nil) list must serialize as [] rather than null. Trident requires
// the field to always be present.
func TestMetadataJsonOsPackagesEmptyListSerializesAsArray(t *testing.T) {
	metadata := MetadataJson{OsPackages: []OsPackage{}}

	data, err := json.Marshal(metadata)
	assert.NoError(t, err)

	assert.Contains(t, string(data), `"osPackages":[]`)
	assert.False(t, strings.Contains(string(data), `"osPackages":null`))
}
