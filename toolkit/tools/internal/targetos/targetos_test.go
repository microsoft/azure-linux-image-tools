// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
	"github.com/stretchr/testify/assert"
)

func TestCleanAclVersionId(t *testing.T) {
	type testDataType struct {
		VersionId             string
		ExpectedCleanVersioId string
	}

	testData := []testDataType{
		{"a", "a"},
		{"1", "1"},
		{"1.2", "1.2"},
		{"1.2.3", "1.2"},
	}

	for _, test := range testData {
		version, _ := version.ParseBasicVersion(test.VersionId)
		cleanVersionId := cleanAclVersionId(test.VersionId, version)
		assert.Equal(t, test.ExpectedCleanVersioId, cleanVersionId)
	}
}
