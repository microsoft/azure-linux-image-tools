# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import argparse

from version_utils import getVersionTags, tryParseVersion

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('version')
    args = parser.parse_args()

    version = tryParseVersion(args.version)
    if version is None:
        raise Exception(f"Failed to parse version ({args.version})")

    versions = getVersionTags(merged=False)

    isLatestVersion = all(version >= other for other in versions)

    if isLatestVersion:
        print("true")
    else:
        print("false")

if __name__ == "__main__":
    main()
