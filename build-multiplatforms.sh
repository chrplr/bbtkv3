#! /bin/bash
# Time-stamp: <2025-02-24 15:33:17 christophe@pallier.org>
#
# Multiplatform Go cross-compilation of all cmd/* commands


if [ "$#" -ne 1 ]
then
	echo "Usage: $0 VERSION (where VERSION is X.Y.Z)"
	exit 1
fi

VERSION=$1
BUILD=$(git rev-parse HEAD)
COMMANDS=$(\ls cmd)
BUILD_FOLDER=./binaries


if [[ -z "${PLATFORMS}" ]]; then
	PLATFORMS="darwin linux windows"
fi

if [[ -z "${ARCHITECTURES}" ]]; then
	ARCHITECTURES="amd64 arm64"
fi


echo Building ${COMMANDS} 
echo for PLATFORMS=${PLATFORMS} 
echo for ARCHITECTURES=${ARCHITECTURES}
echo executables will be in "${BUILD_FOLDER}"
echo 

for OS in $PLATFORMS;
do
    for ARCH in $ARCHITECTURES;
    do
	for CMD in $COMMANDS;
	do
	    export GOOS=${OS}
	    export GOARCH=${ARCH}
	    go build -o ${BUILD_FOLDER}/${CMD}-${GOOS}-${GOARCH}-${VERSION} -ldflags="-X main.Version=${VERSION} -X main.Build=${BUILD}" ./cmd/${CMD}
        done
    done    
done
