package imagecustomizerlib

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageManifestCreate(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)
	testTmpDir := filepath.Join(tmpDir, t.Name())
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	rpmArch := packageManifestRpmArch(t)

	configFile := filepath.Join(testDir, "packagemanifest-create.yaml")
	err := CustomizeImageWithConfigFile(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	require.NoError(t, err)
	verifyPackageManifest(t, buildDir, outImageFilePath, baseImageInfo.MountPoints,
		"azurelinux",                             /* name */
		[]string{"tree-1.8.0-2.azl3." + rpmArch}, /* expectedNevras */
		[]string{"bash", "filesystem", "glibc", "systemd", "rpm", "tdnf"}, /* expectedNames */
		nil, /* expectedAbsentNames */
	)
}

func TestPackageManifestCreateWithPackageManagerRemoval(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)
	testTmpDir := filepath.Join(tmpDir, t.Name())
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	outManifestFilePath := filepath.Join(testTmpDir, "package-manifest.spdx.json")
	rpmArch := packageManifestRpmArch(t)

	configFile := filepath.Join(testDir, "packagemanifest-create-remove-package-manager.yaml")
	err := CustomizeImageWithConfigFile(t.Context(), configFile, ImageCustomizerOptions{
		OutputPackageManifestFile: outManifestFilePath,
		BuildDir:                  buildDir,
		InputImageFile:            baseImage,
		OutputImageFile:           outImageFilePath,
		OutputImageFormat:         "raw",
		UseBaseImageRpmRepos:      true,
		PreviewFeatures:           baseImageInfo.PreviewFeatures,
	})
	require.NoError(t, err)
	manifestBytes := verifyPackageManifest(t, buildDir, outImageFilePath, baseImageInfo.MountPoints,
		"azurelinux", /* name */
		[]string{"tree-1.8.0-2.azl3." + rpmArch, "dos2unix-7.5.1-1.azl3." + rpmArch}, /* expectedNevras */
		[]string{"bash", "filesystem", "glibc", "systemd"},                           /* expectedNames */
		[]string{"rpm", "tdnf"}, /* expectedAbsentNames */
	)

	outBytes, err := os.ReadFile(outManifestFilePath)
	assert.NoError(t, err)
	assert.Equal(t, manifestBytes, outBytes)
}

func TestFinalizePackageManagementPassthroughPreservesBytes(t *testing.T) {
	manifest := "not JSON\n"
	imageChroot := chrootWithManifest(t, manifest)
	err := finalizePackageManagement(t.Context(), nil, imageChroot, nil, "", "passthrough", false)
	assert.NoError(t, err)
	actual, err := os.ReadFile(filepath.Join(imageChroot.RootDir(), packageManifestPath))
	assert.NoError(t, err)
	assert.Equal(t, manifest, string(actual))
}

func TestFinalizePackageManagementNoneDeletesExistingManifest(t *testing.T) {
	imageChroot := chrootWithManifest(t, "old manifest")
	err := finalizePackageManagement(t.Context(), nil, imageChroot, nil, "", "none", false)
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(imageChroot.RootDir(), packageManifestPath))
	assert.ErrorIs(t, err, os.ErrNotExist)
	err = finalizePackageManagement(t.Context(), nil, imageChroot, nil, "", "none", false)
	assert.NoError(t, err)
}

func TestValidateBaseImagePackageManifestExistingBaseRequiresExplicitDecision(t *testing.T) {
	imageChroot := chrootWithManifest(t, "old manifest")
	err := validateBaseImagePackageManifest(&ResolvedConfig{}, imageChroot.RootDir())
	assert.ErrorIs(t, err, ErrPackageManifestModeRequired)
}

func TestValidateBaseImagePackageManifestCannotExportAbsentPassthrough(t *testing.T) {
	rc := &ResolvedConfig{
		PackageManifestMode:       "passthrough",
		OutputPackageManifestPath: filepath.Join(t.TempDir(), "manifest.json"),
	}
	err := validateBaseImagePackageManifest(rc, t.TempDir())
	assert.ErrorIs(t, err, ErrPackageManifestCreateRequired)
}

func TestSubtractPackages(t *testing.T) {
	packageInfo := spdxmanifest.Package{
		ID:      "package",
		Name:    "package",
		Version: "1.0",
		Vendor:  "First Vendor",
		Purl:    "pkg:generic/package@1.0",
	}
	relatedPackage := spdxmanifest.Package{
		ID:   "package-tools",
		Name: "tools",
	}
	otherPackage := spdxmanifest.Package{
		ID:      "other",
		Name:    "other",
		Version: "2.0",
		Vendor:  "Second Vendor",
		Purl:    "pkg:generic/other@2.0",
	}
	duplicatePackage := packageInfo
	duplicatePackage.Vendor = "Different Vendor"
	tests := []struct {
		name        string
		installed   []spdxmanifest.Package
		removed     []string
		expected    []spdxmanifest.Package
		expectedErr error
	}{
		{
			name:     "empty installed packages",
			expected: []spdxmanifest.Package{},
		},
		{
			name:      "no removals",
			installed: []spdxmanifest.Package{relatedPackage, packageInfo, otherPackage},
			expected:  []spdxmanifest.Package{relatedPackage, packageInfo, otherPackage},
		},
		{
			name:      "removes exact IDs",
			installed: []spdxmanifest.Package{relatedPackage, packageInfo, otherPackage},
			removed:   []string{"package"},
			expected:  []spdxmanifest.Package{relatedPackage, otherPackage},
		},
		{
			name:      "all packages removed",
			installed: []spdxmanifest.Package{packageInfo, otherPackage},
			removed:   []string{"other", "package"},
			expected:  []spdxmanifest.Package{},
		},
		{
			name:      "repeated removal IDs",
			installed: []spdxmanifest.Package{relatedPackage, packageInfo, otherPackage},
			removed:   []string{"package", "package"},
			expected:  []spdxmanifest.Package{relatedPackage, otherPackage},
		},
		{
			name:        "unknown removal ID returns no partial result",
			installed:   []spdxmanifest.Package{packageInfo, otherPackage},
			removed:     []string{"package", "missing"},
			expectedErr: ErrPackageManifestRemovalNotInstalled,
		},
		{
			name:        "cannot remove from empty installed packages",
			removed:     []string{"package"},
			expectedErr: ErrPackageManifestRemovalNotInstalled,
		},
		{
			name:        "duplicate installed ID with different metadata",
			installed:   []spdxmanifest.Package{packageInfo, duplicatePackage},
			expectedErr: ErrPackageManifestDuplicatePackage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalInstalled := slices.Clone(test.installed)
			originalRemoved := slices.Clone(test.removed)
			remaining, err := subtractPackages(test.installed, test.removed)

			assert.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, test.expected, remaining)
			assert.Equal(t, originalRemoved, test.removed)
			assert.Equal(t, originalInstalled, test.installed)

			if len(remaining) > 0 {
				remaining[0].Name = "modified"
				assert.Equal(t, originalInstalled, test.installed)
			}
		})
	}
}

func chrootWithManifest(t *testing.T, manifest string) *safechroot.Chroot {
	t.Helper()
	rootDir := t.TempDir()
	manifestPath := filepath.Join(rootDir, packageManifestPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))
	return safechroot.NewChroot(rootDir, true)
}

func packageManifestRpmArch(t *testing.T) string {
	t.Helper()
	rpmArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	require.NotEmpty(t, rpmArch, "unsupported architecture: %s", runtime.GOARCH)
	return rpmArch
}

func verifyPackageManifest(t *testing.T, buildDir string, imageFilePath string, mountPoints []testutils.MountPoint,
	name string, expectedNevras []string,
	expectedNames []string, expectedAbsentNames []string,
) []byte {
	t.Helper()
	imageConnection, err := testutils.ConnectToImage(buildDir, imageFilePath, true, mountPoints)
	require.NoError(t, err)
	defer imageConnection.Close()
	rootDir := imageConnection.Chroot().RootDir()

	manifestBytes, err := os.ReadFile(filepath.Join(rootDir, packageManifestPath))
	require.NoError(t, err)

	osRelease, err := envfile.ParseEnvFile(filepath.Join(rootDir, "etc/os-release"))
	if os.IsNotExist(err) {
		osRelease, err = envfile.ParseEnvFile(filepath.Join(rootDir, "usr/lib/os-release"))
	}
	require.NoError(t, err)

	expectedVersion := osRelease["VERSION"]
	assert.NotEmpty(t, expectedVersion)
	if buildID := osRelease["BUILD_ID"]; buildID != "" {
		expectedVersion += "+" + buildID
	}

	var document struct {
		Name              string `json:"name"`
		DocumentNamespace string `json:"documentNamespace"`
		CreationInfo      struct {
			Created  string   `json:"created"`
			Creators []string `json:"creators"`
		} `json:"creationInfo"`
		Packages []struct {
			SPDXID           string `json:"SPDXID"`
			Name             string `json:"name"`
			VersionInfo      string `json:"versionInfo"`
			Supplier         string `json:"supplier"`
			DownloadLocation string `json:"downloadLocation"`
			FilesAnalyzed    bool   `json:"filesAnalyzed"`
			LicenseConcluded string `json:"licenseConcluded"`
			LicenseDeclared  string `json:"licenseDeclared"`
			CopyrightText    string `json:"copyrightText"`
			ExternalRefs     []struct {
				ReferenceCategory string `json:"referenceCategory"`
				ReferenceType     string `json:"referenceType"`
				ReferenceLocator  string `json:"referenceLocator"`
			} `json:"externalRefs"`
		} `json:"packages"`
	}
	require.NoError(t, json.Unmarshal(manifestBytes, &document))

	assert.Equal(t, name, document.Name)
	assert.NotEmpty(t, document.DocumentNamespace)
	assert.Equal(t, len(document.CreationInfo.Creators), 1)
	_, err = time.Parse(time.RFC3339, document.CreationInfo.Created)
	assert.NoError(t, err)

	rootCount := 0
	actualNevras := []string{}
	packageNames := []string{}
	for _, pkg := range document.Packages {
		if pkg.SPDXID == "SPDXRef-DocumentRoot" {
			rootCount++
			assert.Equal(t, name, pkg.Name)
			assert.Equal(t, "NOASSERTION", pkg.Supplier)
			assert.Equal(t, expectedVersion, pkg.VersionInfo)
			continue
		}

		if !assert.Len(t, pkg.ExternalRefs, 1, pkg.Name) {
			continue
		}

		packageURL, err := url.Parse(pkg.ExternalRefs[0].ReferenceLocator)
		if !assert.NoError(t, err, pkg.Name) {
			continue
		}

		qualifiers, err := url.ParseQuery(packageURL.RawQuery)
		if !assert.NoError(t, err, pkg.Name) {
			continue
		}

		packageIdentity, err := url.PathUnescape(packageURL.Opaque)
		assert.NoError(t, err, pkg.Name)

		rpmPackage, err := newRpmPackageFromNevra(pkg.Name+"-"+pkg.VersionInfo+"."+qualifiers.Get("arch"), "")
		if !assert.NoError(t, err, pkg.Name) {
			continue
		}

		assert.Equal(t, packageIdentity,
			fmt.Sprintf("rpm/%s/%s@%s-%s", name, rpmPackage.Name, rpmPackage.Version, rpmPackage.Release))
		assert.Equal(t, rpmPackage.Epoch, qualifiers.Get("epoch"), pkg.Name)

		nevra := rpmPackage.Nevra()
		assert.NotContains(t, actualNevras, nevra, "duplicate package")
		actualNevras = append(actualNevras, nevra)

		packageNames = append(packageNames, pkg.Name)

	}

	assert.Equal(t, 1, rootCount)

	for _, expectedNevra := range expectedNevras {
		assert.Contains(t, actualNevras, expectedNevra)
	}

	for _, packageName := range expectedNames {
		assert.Contains(t, packageNames, packageName)
	}

	for _, packageName := range expectedAbsentNames {
		assert.NotContains(t, packageNames, packageName)
	}

	return manifestBytes
}

func TestBuildMatchesAclGoldenManifest(t *testing.T) {
	packages := goldenPackages(t)
	assert.Len(t, packages, 22)

	options := spdxmanifest.BuildOptions{
		Name:        "azurecontainerlinux",
		VersionInfo: "0.0.0-spec-conformance",
		ToolVersion: "dev",
		Created:     "2025-01-01T00:00:00Z",
	}
	manifest, err := spdxmanifest.Build(options, packages)
	assert.NoError(t, err)

	actual := map[string]any{}
	require.NoError(t, json.Unmarshal(manifest, &actual))
	expected := map[string]any{}
	require.NoError(t, json.Unmarshal(goldenManifest(t), &expected))
	assert.Equal(t, expected, actual)

	// Check that this PURL contains a literal '&' in the serialized JSON. Unmarshaling treats
	// '&' and '\u0026' identically, so the decoded comparison above cannot check this formatting.
	assert.Contains(t, string(manifest), `"pkg:rpm/azurelinux/zlib@1.3.1-1.azl3?arch=x86_64&epoch=1"`)

	// Check that no ampersand is escaped as '\u0026' anywhere in the document, beyond this PURL.
	// This enforces an output formatting preference, since either spelling is valid, equivalent JSON.
	assert.NotContains(t, string(manifest), `\u0026`)
}

// goldenPackages is the package set of the ACL repo's manifest fixture, in the layout
// `rpm -qa --qf rpmQueryFormat` emits.
func goldenPackages(t *testing.T) []spdxmanifest.Package {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("../../internal/spdxmanifest/testdata", "acl-installed-rpm-packages.txt"))
	require.NoError(t, err)

	packages, err := parseRpmPackageList(string(content))
	require.NoError(t, err)

	manifestPackages := make([]spdxmanifest.Package, 0, len(packages))
	for _, packageInfo := range packages {
		manifestPackages = append(manifestPackages, newManifestPackageFromRpmPackage(packageInfo, "azurelinux"))
	}
	return manifestPackages
}

func goldenManifest(t *testing.T) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("../../internal/spdxmanifest/testdata", "expected-manifest.spdx.json"))
	require.NoError(t, err)

	return content
}
