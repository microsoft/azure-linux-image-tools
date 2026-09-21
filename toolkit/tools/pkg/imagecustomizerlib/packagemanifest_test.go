package imagecustomizerlib

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rpmdb "github.com/anchore/go-rpmdb/pkg"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageManifestCreate(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testPackageManifestCreate(t, baseImageInfo)
		})
	}
}

func testPackageManifestCreate(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+1", 60*60)
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	testTmpDir := filepath.Join(tmpDir, t.Name())
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	configFileName := "packagemanifest-create-azl3.yaml"
	packageManager := "tdnf"
	expectedPackages := map[string]string{
		"bash":       "",
		"filesystem": "",
		"glibc":      "",
		"rpm":        "",
		"systemd":    "",
		"tree":       "1.8.0-2.azl3",
	}
	if baseImageInfo.Version == baseImageVersionAzl4 {
		configFileName = "packagemanifest-create-azl4.yaml"
		packageManager = "dnf5"
		expectedPackages["tree"] = "2.2.1-3.azl4"
	}
	expectedPackages[packageManager] = ""
	configFile := filepath.Join(testDir, configFileName)
	buildStartTime := time.Now().UTC().Truncate(time.Second)
	err := CustomizeImageWithConfigFile(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	buildEndTime := time.Now().UTC()
	require.NoError(t, err)

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true, baseImageInfo.MountPoints)
	require.NoError(t, err)
	defer imageConnection.Close()
	rootDir := imageConnection.Chroot().RootDir()

	_, manifestPackages := verifyPackageManifest(t, rootDir, baseImageInfo.Distro /* name */, expectedPackages,
		nil /* expectedAbsentNames */, buildStartTime, buildEndTime,
	)
	actualNevras := verifyRpmManifestPackages(t, manifestPackages)

	rpmQueryOutput, _, err := shell.
		NewExecBuilder("rpm", "-qa", "--queryformat", `%|ARCH?{%{NEVRA}\n}:{}|`).
		Chroot(rootDir).
		ExecuteCaptureOutput()
	require.NoError(t, err)
	expectedNevras := strings.Split(strings.TrimSpace(rpmQueryOutput), "\n")

	assert.ElementsMatch(t, expectedNevras, actualNevras)
}

func TestPackageManifestCreateWithPackageManagerRemoval(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testPackageManifestCreateWithPackageManagerRemoval(t, baseImageInfo)
		})
	}
}

func testPackageManifestCreateWithPackageManagerRemoval(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+1", 60*60)
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	testTmpDir := filepath.Join(tmpDir, t.Name())
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	outManifestFilePath := filepath.Join(testTmpDir, "package-manifest.spdx.json")

	configFileName := "packagemanifest-create-remove-package-manager-azl3.yaml"
	expectedPackages := map[string]string{
		"bash":       "",
		"dos2unix":   "7.5.1-1.azl3",
		"filesystem": "",
		"glibc":      "",
		"systemd":    "",
		"tree":       "1.8.0-2.azl3",
	}
	if baseImageInfo.Version == baseImageVersionAzl4 {
		configFileName = "packagemanifest-create-remove-package-manager-azl4.yaml"
		expectedPackages["tree"] = "2.2.1-3.azl4"
		expectedPackages["dos2unix"] = "7.5.3-2.azl4"
	}
	configFile := filepath.Join(testDir, configFileName)
	buildStartTime := time.Now().UTC().Truncate(time.Second)
	err := CustomizeImageWithConfigFile(t.Context(), configFile, ImageCustomizerOptions{
		OutputPackageManifestFile: outManifestFilePath,
		BuildDir:                  buildDir,
		InputImageFile:            baseImage,
		OutputImageFile:           outImageFilePath,
		OutputImageFormat:         "raw",
		UseBaseImageRpmRepos:      true,
		PreviewFeatures:           baseImageInfo.PreviewFeatures,
	})
	buildEndTime := time.Now().UTC()
	require.NoError(t, err)

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true, baseImageInfo.MountPoints)
	require.NoError(t, err)
	defer imageConnection.Close()
	rootDir := imageConnection.Chroot().RootDir()

	manifestBytes, manifestPackages := verifyPackageManifest(t, rootDir, baseImageInfo.Distro, /* name */
		expectedPackages, []string{"rpm", "tdnf", "dnf5"} /* expectedAbsentNames */, buildStartTime, buildEndTime,
	)
	verifyRpmManifestPackages(t, manifestPackages)

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

func chrootWithManifest(t *testing.T, manifest string) *safechroot.Chroot {
	t.Helper()
	rootDir := t.TempDir()
	manifestPath := filepath.Join(rootDir, packageManifestPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))
	return safechroot.NewChroot(rootDir, true)
}

func verifyPackageManifest(t *testing.T, rootDir string, name string, expectedPackages map[string]string,
	expectedAbsentNames []string, buildStartTime time.Time, buildEndTime time.Time,
) ([]byte, []*spdx.Package) {
	t.Helper()

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

	var document spdx.Document
	require.NoError(t, json.Unmarshal(manifestBytes, &document))

	assert.Equal(t, name, document.DocumentName)
	assert.NotEmpty(t, document.DocumentNamespace)
	require.NotNil(t, document.CreationInfo)
	assert.Len(t, document.CreationInfo.Creators, 1)

	createdTime, err := time.Parse(time.RFC3339, document.CreationInfo.Created)
	require.NoError(t, err)
	assert.Equal(t, time.UTC, createdTime.Location())
	assert.WithinRange(t, createdTime, buildStartTime, buildEndTime)

	rootCount := 0
	packageNames := []string{}
	manifestPackages := []*spdx.Package{}
	for _, manifestPackage := range document.Packages {
		if manifestPackage.PackageSPDXIdentifier == "DocumentRoot" {
			rootCount++
			assert.Equal(t, name, manifestPackage.PackageName)
			require.NotNil(t, manifestPackage.PackageSupplier)
			assert.Equal(t, "NOASSERTION", manifestPackage.PackageSupplier.Supplier)
			assert.Equal(t, expectedVersion, manifestPackage.PackageVersion)
			continue
		}

		packageNames = append(packageNames, manifestPackage.PackageName)
		if expectedVersion := expectedPackages[manifestPackage.PackageName]; expectedVersion != "" {
			assert.Equal(t, expectedVersion, manifestPackage.PackageVersion, manifestPackage.PackageName)
		}

		if !assert.Len(t, manifestPackage.PackageExternalReferences, 1, manifestPackage.PackageName) {
			continue
		}
		externalReference := manifestPackage.PackageExternalReferences[0]
		assert.Equal(t, "PACKAGE-MANAGER", externalReference.Category, manifestPackage.PackageName)
		assert.Equal(t, "purl", externalReference.RefType, manifestPackage.PackageName)

		packageURL, err := url.Parse(externalReference.Locator)
		if !assert.NoError(t, err, manifestPackage.PackageName) {
			continue
		}
		assert.Equal(t, "pkg", packageURL.Scheme, manifestPackage.PackageName)
		assert.NotEmpty(t, packageURL.Opaque, manifestPackage.PackageName)
		manifestPackages = append(manifestPackages, manifestPackage)
	}

	assert.Equal(t, 1, rootCount)
	for packageName := range expectedPackages {
		assert.Contains(t, packageNames, packageName)
	}
	for _, packageName := range expectedAbsentNames {
		assert.NotContains(t, packageNames, packageName)
	}

	return manifestBytes, manifestPackages
}

func verifyRpmManifestPackages(t *testing.T, packages []*spdx.Package) []string {
	t.Helper()
	nevras := []string{}
	for _, pkg := range packages {
		require.Len(t, pkg.PackageExternalReferences, 1, pkg.PackageName)
		packageURL, err := url.Parse(pkg.PackageExternalReferences[0].Locator)
		require.NoError(t, err, pkg.PackageName)

		qualifiers, err := url.ParseQuery(packageURL.RawQuery)
		if !assert.NoError(t, err, pkg.PackageName) {
			continue
		}
		assert.NotEmpty(t, qualifiers.Get("arch"), pkg.PackageName)

		nevra := pkg.PackageName + "-" + pkg.PackageVersion + "." + qualifiers.Get("arch")
		assert.NotContains(t, nevras, nevra, "duplicate package")
		nevras = append(nevras, nevra)
	}

	return nevras
}

func TestNewManifestPackageFromRpmPackagePurl(testContext *testing.T) {
	epoch := 2
	zeroEpoch := 0
	testCases := []struct {
		name        string
		namespace   string
		packageName string
		epoch       *int
		expected    string
	}{
		{
			name:        "plain",
			namespace:   "azurelinux",
			packageName: "example",
			expected:    "pkg:rpm/azurelinux/example@1.0-3?arch=x86_64",
		},
		{
			name:        "epoch",
			namespace:   "azurelinux",
			packageName: "example",
			epoch:       &epoch,
			expected:    "pkg:rpm/azurelinux/example@1.0-3?arch=x86_64&epoch=2",
		},
		{
			name:        "zero epoch",
			namespace:   "azurelinux",
			packageName: "example",
			epoch:       &zeroEpoch,
			expected:    "pkg:rpm/azurelinux/example@1.0-3?arch=x86_64&epoch=0",
		},
		{
			name:        "spaces",
			namespace:   "vendor name",
			packageName: "package name",
			expected:    "pkg:rpm/vendor%20name/package%20name@1.0-3?arch=x86_64",
		},
		{
			name:        "reserved characters",
			namespace:   "vendor+name",
			packageName: "package@name",
			expected:    "pkg:rpm/vendor%2Bname/package%40name@1.0-3?arch=x86_64",
		},
		{
			name:        "colons",
			namespace:   "vendor:name",
			packageName: "package:name",
			expected:    "pkg:rpm/vendor:name/package:name@1.0-3?arch=x86_64",
		},
	}

	for _, testCase := range testCases {
		testContext.Run(testCase.name, func(testContext *testing.T) {
			packageInfo := rpmdb.PackageInfo{
				Name:    testCase.packageName,
				Epoch:   testCase.epoch,
				Version: "1.0",
				Release: "3",
				Arch:    "x86_64",
			}
			manifestPackage, err := newManifestPackageFromRpmPackage(packageInfo, testCase.namespace)
			require.NoError(testContext, err)
			assert.Equal(testContext, testCase.expected, manifestPackage.Purl)
		})
	}
}

func TestBuildMatchesAclGoldenManifest(t *testing.T) {
	packages := goldenPackages(t)
	assert.Len(t, packages, 22)

	options := spdxmanifest.BuildMetadata{
		Name:        "azurecontainerlinux",
		VersionInfo: "0.0.0-spec-conformance",
		ToolVersion: "dev",
		Created:     "2025-01-01T00:00:00Z",
	}
	manifest, err := spdxmanifest.Build(options, packages)
	assert.NoError(t, err)

	actual := map[string]any{}
	require.NoError(t, json.Unmarshal(manifest, &actual))
	expectedManifest := goldenManifest(t)
	expected := map[string]any{}
	require.NoError(t, json.Unmarshal(expectedManifest, &expected))
	assert.Equal(t, expected, actual)

	var document spdx.Document
	require.NoError(t, json.Unmarshal(manifest, &document))
	require.Len(t, document.Packages, len(packages)+1)
	assert.Len(t, verifyRpmManifestPackages(t, document.Packages[1:]), len(packages))

	// Check that this PURL contains a literal '&' in the serialized JSON. Unmarshaling treats
	// '&' and '\u0026' identically, so the decoded comparison above cannot check this formatting.
	assert.Contains(t, string(manifest), `"pkg:rpm/azurelinux/zlib@1.3.1-1.azl3?arch=x86_64&epoch=1"`)

	// Check that no ampersand is escaped as '\u0026' anywhere in the document, beyond this PURL.
	// This enforces an output formatting preference, since either spelling is valid, equivalent JSON.
	assert.NotContains(t, string(manifest), `\u0026`)
}

func goldenPackages(t *testing.T) []spdxmanifest.Package {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("../../internal/spdxmanifest/testdata", "expected-packages.json"))
	require.NoError(t, err)

	var packages []rpmdb.PackageInfo
	require.NoError(t, json.Unmarshal(content, &packages))

	manifestPackages := make([]spdxmanifest.Package, 0, len(packages))
	for _, packageInfo := range packages {
		manifestPackage, err := newManifestPackageFromRpmPackage(packageInfo, "azurelinux")
		require.NoError(t, err)
		manifestPackages = append(manifestPackages, manifestPackage)
	}
	return manifestPackages
}

func goldenManifest(t *testing.T) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("../../internal/spdxmanifest/testdata", "expected-manifest.spdx.json"))
	require.NoError(t, err)

	return content
}
