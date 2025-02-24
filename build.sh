#! /bin/bash
# Time-stamp: <2025-02-24 15:33:17 christophe@pallier.org>
#
# Multiplatform Go cross-compilation of all cmd/* commands

VERSION=1.0.1
BUILD=$(git rev-parse HEAD)
PLATFORMS="darwin linux windows"
ARCHITECTURES="amd64 arm64"
LDFLAGS="-ldflags \"-X main.Version=${VERSION} -X main.Build=${BUILD}\""


set +x

for OS in $PLATFORMS;
do
    for ARCH in $ARCHITECTURES;
    do
	for CMD in $(ls cmd);
	do
	    export GOOS=${OS}
	    export GOARCH=${ARCH}
	    go build  -o binaries/${CMD}-${GOOS}-${GOARCH} ./cmd/${CMD}
        done
    done    
done
