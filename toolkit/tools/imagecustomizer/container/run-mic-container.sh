#!/bin/bash

set -e
set -x

function showUsage() {
    echo
    echo "usage:"
    echo
    echo "run-mic-container.sh \\"
    echo "    -t <container-tag> \\"
    echo "    -i <input-image-path> \\"
    echo "    -c <input-config-path> \\"
    echo "    -f <output-format> \\"
    echo "    -o <output-image-path> \\"
    echo "   [ -g <generated-hash-files-path> ] \\"
    echo "   [ -s <signed-hash-file-path>] \\"
    echo "   [-l <log-level>"]
    echo
}

while getopts ":r:n:t:i:c:f:o:l:g:s:" OPTIONS; do
  case "${OPTIONS}" in
    t ) containerTag=$OPTARG ;;
    i ) inputImage=$OPTARG ;;
    c ) inputConfig=$OPTARG ;;
    f ) outputFormat=$OPTARG ;;
    o ) outputImage=$OPTARG ;;
    g ) generatedHashFilesPath=$OPTARG ;;
    s ) signedHashFilePath=$OPTARG ;;
    l ) logLevel=$OPTARG ;;
  esac
done

if [[ -z $containerTag ]]; then
    echo "missing required argument '-t containerTag'"
    showUsage
    exit 1
fi

if [[ -z $inputImage ]]; then
    echo "missing required argument '-i inputImage'"
    showUsage
    exit 1
fi

if [[ -z $inputConfig ]]; then
    echo "missing required argument '-c inputConfig'"
    showUsage
    exit 1
fi

if [[ -z $outputFormat ]]; then
    echo "missing required argument '-f outputFormat'"
    showUsage
    exit 1
fi

if [[ -z $outputImage ]]; then
    echo "missing required argument '-o outputImage'"
    showUsage
    exit 1
fi

if [[ -z $logLevel ]]; then
    logLevel=info
fi

# ---- main ----

inputImageDir=$(dirname $inputImage)
inputConfigDir=$(dirname $inputConfig)
outputImageDir=$(dirname $outputImage)

mkdir -p $outputImageDir

# setup input image within the container
containerInputImageDir=/mic/input
containerInputImage=$containerInputImageDir/$(basename $inputImage)

# setup input config within the container
containerInputConfigDir=/mic/config
containerInputConfig=$containerInputConfigDir/$(basename $inputConfig)

# setup build folder within the container
containerBuildDir=/mic/build

# setup output image within the container
containerOutputDir=/mic/output
containerOutputImage=$containerOutputDir/$(basename $outputImage)

dockerVeritySignatureParameters=()
veritySignatureParameters=()

if [[ -n "$generatedHashFilesPath" ]]; then

    mkdir -p $generatedHashFilesPath
    containerGeneratedHashFilesPath=/mic/exported-hashes

    dockerVeritySignatureParameters+=("-v" "$generatedHashFilesPath:$containerGeneratedHashFilesPath:z")

    veritySignatureParameters+=("--output-verity-hashes")
    veritySignatureParameters+=("--output-verity-hashes-dir" "$containerGeneratedHashFilesPath")
    veritySignatureParameters+=("--require-signed-root-hashes")
    veritySignatureParameters+=("--require-signed-rootfs-root-hash")
fi

if [[ -n "$signedHashFilePath" ]]; then
    signedHashFileDirPath=$(dirname $signedHashFilePath)
    signedHashFile=$(basename $signedHashFilePath)
    containerSignedHashFileDirPath=/mic/signed-hashes
    containerSignedHashFilePath=$containerSignedHashFileDirPath/$signedHashFile

    dockerVeritySignatureParameters+=("-v" "$signedHashFileDirPath:$containerSignedHashFileDirPath:z")

    veritySignatureParameters+=("--input-signed-verity-hashes-files" "$containerSignedHashFilePath")
fi

# invoke
docker run --rm \
  --privileged=true \
   -v $inputImageDir:$containerInputImageDir:z \
   -v $inputConfigDir:$containerInputConfigDir:z \
   -v $outputImageDir:$containerOutputDir:z \
   "${dockerVeritySignatureParameters[@]}" \
   -v /dev:/dev \
   "$containerTag" \
   imagecustomizer \
      --image-file $containerInputImage \
      --config-file $containerInputConfig \
      --build-dir $containerBuildDir \
      --output-image-format $outputFormat \
      --output-image-file $containerOutputImage \
      "${veritySignatureParameters[@]}" \
      --log-level $logLevel
