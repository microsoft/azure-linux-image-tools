# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import sys

from version_utils import getMakefileVersion, getVersionTags

def main():
    makefileVersion = getMakefileVersion()

    versions = getVersionTags(merged=True)
    if len(versions) <= 0:
        print(f"No previous tags found", file=sys.stderr)
        exit(2)

    mostRecentVersion = versions[0]

    if mostRecentVersion[0] != makefileVersion[0] or mostRecentVersion[1] != makefileVersion[1]:
        print(f"Makefile and most recent major/minor versions don't match: {makefileVersion} vs. {mostRecentVersion}", file=sys.stderr)
        exit(3)

    newPatchVersion = (mostRecentVersion[0], mostRecentVersion[1], mostRecentVersion[2]+1)
    print(f"{newPatchVersion[0]}.{newPatchVersion[1]}.{newPatchVersion[2]}")

if __name__ == "__main__":
    main()
