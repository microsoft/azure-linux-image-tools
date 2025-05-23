import os
from pathlib import Path
import re
import subprocess
import sys

SCRIPT_DIR = Path(os.path.realpath(__file__)).parent
REPO_DIR = (SCRIPT_DIR / "../../../").resolve()
VERSION_MAKEFILE_PATH = REPO_DIR / "toolkit/scripts/build_tag_imagecustomizer.mk"

def main():
    makefileVersion = getMakefileVersion()

    versions = getVersionTags()
    if len(versions) <= 0:
        print(f"No previous tags found", file=sys.stderr)
        exit(2)

    mostRecentVersion = versions[0]

    if mostRecentVersion[0] != makefileVersion[0] or mostRecentVersion[1] != makefileVersion[1]:
        print(f"Makefile and most recent major/minor version s don't match: {makefileVersion} vs. {mostRecentVersion}", file=sys.stderr)
        exit(3)

    newPatchVersion = (mostRecentVersion[0], mostRecentVersion[1], mostRecentVersion[2]+1)
    print(f"{newPatchVersion[0]}.{newPatchVersion[1]}.{newPatchVersion[2]}")

def getMakefileVersion():
    with open(VERSION_MAKEFILE_PATH, "r") as file:
        version_makefile = file.read()

    makefileRegex = re.compile(r"^IMAGE_CUSTOMIZER_VERSION \?= ([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)

    match = makefileRegex.search(version_makefile)
    if match is None:
        print(f"Failed to parse makefile ({VERSION_MAKEFILE_PATH})")
        exit(1)

    makefileVersion = (int(match.group(1)), int(match.group(2)), int(match.group(3)))
    return makefileVersion

def getVersionTags():
    tagListProc = subprocess.run(
        ["git", "-C", REPO_DIR, "tag", "--list", "v*", "--merged"],
        text=True, check=True, capture_output=True)
    strTags = tagListProc.stdout.splitlines()

    versionRegex = re.compile(r"^v([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)

    versions=[]
    for strTag in strTags:
        match = versionRegex.match(strTag)
        if match is None:
            continue

        version = (int(match.group(1)), int(match.group(2)), int(match.group(3)))
        versions.append(version)

    versions.sort(reverse=True)
    return versions

if __name__ == "__main__":
    main()
