import os
from pathlib import Path
import re

SCRIPT_DIR = Path(os.path.realpath(__file__)).parent
REPO_DIR = (SCRIPT_DIR / "../../../").resolve()
VERSION_MAKEFILE_PATH = REPO_DIR / "toolkit/scripts/build_tag_imagecustomizer.mk"

def main():
    with open(VERSION_MAKEFILE_PATH, "r") as file:
        version_makefile = file.read()

    regex = re.compile(r"^image_customizer_version \?= ([0-9]+)\.([0-9]+)\.([0-9]+)$", re.MULTILINE)

    match = regex.search(version_makefile)
    if match is None:
        raise RuntimeError(f"Failed to parse makefile ({VERSION_MAKEFILE_PATH})")

    major = int(match.group(1))
    minor = int(match.group(2))
    patch = int(match.group(3))

    # Bump version
    minor += 1
    patch = 0

    version_makefile = regex.sub(f"image_customizer_version ?= {major}.{minor}.{patch}", version_makefile, count=1)

    with open(VERSION_MAKEFILE_PATH, "w") as file:
        file.write(version_makefile)

    print(f"{major}.{minor}.{patch}")

if __name__ == "__main__":
    main()
