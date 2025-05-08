#!/bin/bash

# set -x
set -e

scriptDir="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
enlistmentRoot="$scriptDir/../../../.."

ARCH="amd64"
ORAS_VERSION="1.2.3"
ORAS_EXPECTED_SHA256="b4efc97a91f471f323f193ea4b4d63d8ff443ca3aab514151a30751330852827"

function showUsage() {
    echo
    echo "usage:"
    echo
    echo "build-container.sh \\"
    echo "    -t <container-tag>"
    echo "    -a <architecture> (default: amd64)"
    echo
}

while getopts ":r:n:t:a:" OPTIONS; do
  case "${OPTIONS}" in
    t ) containerTag=$OPTARG ;;
    a ) ARCH=$OPTARG ;;
    \? ) echo "Invalid option: $OPTARG" 1>&2; showUsage; exit 1 ;;
  esac
done

# only supported values for architecture are amd64 and arm64
if [[ $ARCH != "amd64" && $ARCH != "arm64" ]]; then
    echo "Unsupported architecture: $ARCH"
    showUsage
    exit 1
fi

if [[ -z $containerTag ]]; then
    echo "missing required argument '-t containerTag'"
    showUsage
    exit 1
fi

# ---- main ----

buildDir="$scriptDir/build"
containerStagingFolder="$buildDir/container"

function cleanUp() {
    local exit_code=$?
    rm -rf "$containerStagingFolder"
    exit $exit_code
}
trap 'cleanUp' ERR

exeFile="$enlistmentRoot/toolkit/out/tools/imagecustomizer"
licensesDir="$enlistmentRoot/toolkit/out/LICENSES"

stagingBinDir="${containerStagingFolder}/usr/local/bin"
stagingLicensesDir="${containerStagingFolder}/usr/local/share/licenses"

dockerFile="$scriptDir/imagecustomizer.Dockerfile"
runScriptPath="$scriptDir/run.sh"

# stage those files that need to be in the container
mkdir -p "${stagingBinDir}"
mkdir -p "${stagingLicensesDir}"

cp "$exeFile" "${stagingBinDir}"
cp "$runScriptPath" "${stagingBinDir}"
cp -R "$licensesDir" "${stagingLicensesDir}/imagecustomizer"

touch ${containerStagingFolder}/.mariner-toolkit-ignore-dockerenv

# download oras
orasUnzipDir="${buildDir}/oras-install-${ORAS_VERSION}/"
if [ ! -d "$orasUnzipDir" ]; then
  ORAS_TAR="${buildDir}/oras_${ORAS_VERSION}_linux_${ARCH}.tar.gz"

  curl -L "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_${ARCH}.tar.gz" \
    -o "$ORAS_TAR"

  ORAS_TAR_SHA256="$(sha256sum "$ORAS_TAR" | cut -d " " -f 1)"
  if [ "$ORAS_TAR_SHA256" != "$ORAS_EXPECTED_SHA256" ]; then
    echo "Incorrect oras tar SHA256: ${ORAS_TAR_SHA256} vs. ${ORAS_EXPECTED_SHA256}"
    exit 1
  fi

  mkdir "$orasUnzipDir"
  tar -zxf "$ORAS_TAR" -C "$orasUnzipDir/"
fi

# stage oras
cp "$orasUnzipDir/oras" "${stagingBinDir}"

ORAS_LICENSES_DIR="${stagingLicensesDir}/oras"
mkdir -p "${ORAS_LICENSES_DIR}"
cp "$orasUnzipDir/LICENSE" "${ORAS_LICENSES_DIR}"

# azl doesn't support grub2-pc for arm64, hence remove it from dockerfile
if [ "$ARCH" == "arm64" ]; then
    echo "Removing grub2-pc and systemd-ukify from Dockerfile for arm64"
    sed -i 's/\<grub2-pc systemd-ukify\>//g' imagecustomizer.Dockerfile
fi

# build the container
docker build -f "$dockerFile" "$containerStagingFolder" -t "$containerTag"

# clean-up
cleanUp
