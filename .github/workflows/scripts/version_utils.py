# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from pathlib import Path
import subprocess
import re

SCRIPT_DIR = Path(os.path.realpath(__file__)).parent
REPO_DIR = (SCRIPT_DIR / "../../../").resolve()
VERSION_MAKEFILE_PATH = REPO_DIR / "toolkit/scripts/build_tag_imagecustomizer.mk"

VERSION_REGEX = re.compile(r"^v([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)
MAKEFILE_VERSION_REGEX = re.compile(r"^IMAGE_CUSTOMIZER_VERSION \?= ([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)

def readMakefileVersion():
    with open(VERSION_MAKEFILE_PATH, "r") as file:
        versionMakefileContents = file.read()
    return versionMakefileContents

def parseMakefileVersion(versionMakefileContents):
    makefileRegex = re.compile(r"^IMAGE_CUSTOMIZER_VERSION \?= ([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)

    match = makefileRegex.search(versionMakefileContents)
    if match is None:
        raise Exception(f"Failed to parse makefile ({VERSION_MAKEFILE_PATH})")

    makefileVersion = (int(match.group(1)), int(match.group(2)), int(match.group(3)))
    return makefileVersion

def getMakefileVersion():
    versionMakefileContents = readMakefileVersion()
    makefileVersion = parseMakefileVersion(versionMakefileContents)
    return makefileVersion

def getVersionTags(merged):
    args = ["git", "-C", REPO_DIR, "tag", "--list", "v*"]
    if merged:
        args.append("--merged")
    
    tagListProc = subprocess.run(args, text=True, check=True, capture_output=True)
    strTags = tagListProc.stdout.splitlines()

    versionRegex = re.compile(r"^v([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)

    versions=[]
    for strTag in strTags:
        version = tryParseVersion(strTag)
        if version is None:
            continue

        versions.append(version)

    versions.sort(reverse=True)
    return versions

def tryParseVersion(versionStr):
    match = VERSION_REGEX.match(versionStr)
    if match is None:
        return None

    version = (int(match.group(1)), int(match.group(2)), int(match.group(3)))
    return version
