# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import sys

from version_utils import getMakefileVersion, getVersionTags

def main():
    makefileVersion = getMakefileVersion()
    makefileVersionMjrMin = makefileVersion[0:2]

    versions = getVersionTags(merged=True)

    mostRecentVersion = None
    mostRecentVersionMjrMin = None
    if len(versions) > 0:
        mostRecentVersion = versions[0]
        mostRecentVersionMjrMin = mostRecentVersion[0:2]

    if mostRecentVersion is not None and makefileVersionMjrMin < mostRecentVersionMjrMin:
        print(f"Makefile major/minor version is less than most recent major/minor version: "+
            f"{makefileVersion} vs. {mostRecentVersion}",
            file=sys.stderr)
        exit(3)

    elif mostRecentVersion is not None and makefileVersionMjrMin == mostRecentVersionMjrMin:
        # A previous version tag for the current major/minor version exists.
        # So, increment the patch version.
        newPatchVersion = (mostRecentVersion[0], mostRecentVersion[1], mostRecentVersion[2]+1)

    else:
        # A version tag doesn't exist yet for the current major/minor version.
        # So, start from patch 0.
        newPatchVersion = makefileVersion

    print(f"{newPatchVersion[0]}.{newPatchVersion[1]}.{newPatchVersion[2]}")

if __name__ == "__main__":
    main()
