#!/bin/bash

set -e

function showUsage() {
    echo
    echo "usage:"
    echo
    echo "run-container.sh \\"
    echo "    -t <container-tag> \\"
    echo "    -i <input-image-path> \\"
    echo "    -c <input-config-path> \\"
    echo "    -f <output-format> \\"
    echo "    -o <output-image-path> \\"
    echo "   [-l <log-level>"]
    echo
}

while getopts ":r:n:t:i:c:f:o:l:" OPTIONS; do
  case "${OPTIONS}" in
    t ) containerTag=$OPTARG ;;
    i ) inputImage=$OPTARG ;;
    c ) inputConfig=$OPTARG ;;
    f ) outputFormat=$OPTARG ;;
    o ) outputImage=$OPTARG ;;
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
containerInputImageDir=/container/input
containerInputImage=$containerInputImageDir/$(basename $inputImage)

# setup input config within the container
containerInputConfigDir=/container/config
containerInputConfig=$containerInputConfigDir/$(basename $inputConfig)

# setup build folder within the container
containerBuildDir=/container/build

# setup output image within the container
containerOutputDir=/container/output
containerOutputImage=$containerOutputDir/$(basename $outputImage)

# invoke
docker run --rm \
  --privileged=true \
   -v $inputImageDir:$containerInputImageDir:z \
   -v $inputConfigDir:$containerInputConfigDir:z \
   -v $outputImageDir:$containerOutputDir:z \
   -v /dev:/dev \
   "$containerTag" \
   imagecustomizer \
      --image-file $containerInputImage \
      --config-file $containerInputConfig \
      --build-dir $containerBuildDir \
      --output-image-format $outputFormat \
      --output-image-file $containerOutputImage \
      --log-level $logLevel
