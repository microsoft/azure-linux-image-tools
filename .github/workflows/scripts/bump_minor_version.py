# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from version_utils import readMakefileVersion, parseMakefileVersion, MAKEFILE_VERSION_REGEX, VERSION_MAKEFILE_PATH

def main():
    versionMakefileContents = readMakefileVersion()
    (major, minor, patch) = parseMakefileVersion(versionMakefileContents)

    # Bump version
    minor += 1
    patch = 0

    versionMakefileContents = MAKEFILE_VERSION_REGEX.sub(f"IMAGE_CUSTOMIZER_VERSION ?= {major}.{minor}.{patch}",
        versionMakefileContents, count=1)

    with open(VERSION_MAKEFILE_PATH, "w") as file:
        file.write(versionMakefileContents)

    print(f"{major}.{minor}.{patch}")

if __name__ == "__main__":
    main()
