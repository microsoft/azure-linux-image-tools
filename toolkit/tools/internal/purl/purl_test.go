package purl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpmPackageURL(testContext *testing.T) {
	epoch := 2
	testCases := []struct {
		name      string
		namespace string
		pkgName   string
		epoch     *int
		expected  string
	}{
		{
			name:      "plain",
			namespace: "azurelinux",
			pkgName:   "example",
			expected:  "pkg:rpm/azurelinux/example@1.0-3?arch=x86_64",
		},
		{
			name:      "epoch",
			namespace: "azurelinux",
			pkgName:   "example",
			epoch:     &epoch,
			expected:  "pkg:rpm/azurelinux/example@1.0-3?arch=x86_64&epoch=2",
		},
		{
			name:      "spaces",
			namespace: "vendor name",
			pkgName:   "package name",
			expected:  "pkg:rpm/vendor%20name/package%20name@1.0-3?arch=x86_64",
		},
		{
			name:      "reserved characters",
			namespace: "vendor+name",
			pkgName:   "package@name",
			expected:  "pkg:rpm/vendor%2Bname/package%40name@1.0-3?arch=x86_64",
		},
		{
			name:      "colons",
			namespace: "vendor:name",
			pkgName:   "package:name",
			expected:  "pkg:rpm/vendor:name/package:name@1.0-3?arch=x86_64",
		},
	}

	for _, testCase := range testCases {
		testContext.Run(testCase.name, func(testContext *testing.T) {
			actual := RpmPackageURL(testCase.namespace, testCase.pkgName, testCase.epoch, "1.0", "3", "x86_64")
			assert.Equal(testContext, testCase.expected, actual)
		})
	}
}
