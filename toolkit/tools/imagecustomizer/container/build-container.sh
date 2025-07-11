#!/bin/bash

# set -x
set -e

scriptDir="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
enlistmentRoot="$scriptDir/../../../.."

ARCH="amd64"
ORAS_VERSION="1.2.2"
BASE_IMAGE="mcr.microsoft.com/azurelinux/base/core"
BASE_IMAGE_TAG="$BASE_IMAGE:3.0"

# Telemetry connection string for Azure Monitor
AZ_CONN_STRING="InstrumentationKey=a65b25be-d5c6-4aca-aa84-5c1c35cc9ab1;IngestionEndpoint=https://eastus2-3.in.applicationinsights.azure.com/;LiveEndpoint=https://eastus2.livediagnostics.monitor.azure.com/;ApplicationId=6faca17c-7478-4f1a-9c38-15765db103cb"

function showUsage() {
    echo
    echo "usage:"
    echo
    echo "build-container.sh \\"
    echo "    -t <container-tag>"
    echo "    -a <architecture> (default: amd64)"
    echo
}

while getopts "t:a:b" OPTIONS; do
  case "${OPTIONS}" in
    t ) containerTag=$OPTARG ;;
    a ) ARCH=$OPTARG ;;
    b ) VERIFY_BASE_IMAGE=true ;;
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

baseImage="$BASE_IMAGE_TAG"

if [ $VERIFY_BASE_IMAGE ]; then
    dockerPullStdout="$(docker image pull "$BASE_IMAGE_TAG")"
    baseImageDigest="$(grep -E 'Digest: sha256:[a-zA-Z0-9]+' <<< "$dockerPullStdout" | cut -d ' ' -f 2)"
    baseImageByDigest="$BASE_IMAGE@$baseImageDigest"

    # Verify the signature of the base image.
    notation verify "$baseImageByDigest"

    baseImage="$baseImageByDigest"
fi

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

telemetryScript="$enlistmentRoot/toolkit/scripts/telemetry_hopper/telemetry_hopper.py"
telemetryRequirements="$enlistmentRoot/toolkit/scripts/telemetry_hopper/requirements.txt"
entrypointScript="$scriptDir/entrypoint.sh"

stagingBinDir="${containerStagingFolder}/usr/local/bin"
stagingLicensesDir="${containerStagingFolder}/usr/local/share/licenses"

dockerFile="$scriptDir/imagecustomizer.Dockerfile"
runScriptPath="$scriptDir/run.sh"

# stage those files that need to be in the container
mkdir -p "${stagingBinDir}"
mkdir -p "${stagingLicensesDir}"

cp "$exeFile" "${stagingBinDir}"
cp "$runScriptPath" "${stagingBinDir}"
cp -R "$licensesDir" "${stagingLicensesDir}"
cp "$telemetryScript" "${stagingBinDir}"
cp "$telemetryRequirements" "${stagingBinDir}"
cp "$entrypointScript" "${stagingBinDir}"

touch ${containerStagingFolder}/.mariner-toolkit-ignore-dockerenv

# download oras
orasUnzipDir="${buildDir}/oras-install/"
if [ ! -d "$orasUnzipDir" ]; then
  ORAS_TAR="${buildDir}/oras_${ORAS_VERSION}_linux_${ARCH}.tar.gz"

  curl -L "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_${ARCH}.tar.gz" \
    -o "$ORAS_TAR"

  mkdir "$orasUnzipDir"
  tar -zxf "$ORAS_TAR" -C "$orasUnzipDir/"
fi

# stage oras
cp "$orasUnzipDir/oras" "${stagingBinDir}"

# azl doesn't support grub2-pc for arm64, hence remove it from dockerfile
if [ "$ARCH" == "arm64" ]; then
    echo "Removing grub2-pc and systemd-ukify from Dockerfile for arm64"
    sed -i 's/\<grub2-pc systemd-ukify\>//g' "$dockerFile"
fi

# build the container
if [[ "$containerTag" == "azurelinuximagecustomizer:latest" ]]; then
    docker build --build-arg "BASE_IMAGE=$baseImage" --build-arg "AZ_CONN_STRING=$AZ_CONN_STRING" -f "$dockerFile" "$containerStagingFolder" -t "$containerTag"
else
    docker build --build-arg "BASE_IMAGE=$baseImage" -f "$dockerFile" "$containerStagingFolder" -t "$containerTag"
fi

# clean-up
cleanUp
