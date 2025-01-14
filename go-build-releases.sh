#!/bin/bash
#
# This script was adapted from: https://gist.github.com/eduncan911/68775dba9d3c028181e4 
# 
# The changes aer as follows:
# - Add the check to see if there are uncommited git changes
# - Add link time variable for the git hash and build time
# - Rearrange the ARM platforms sections
# - Some platforms are disabled by commenting them out
#
# The link time variables section should be reverted if your program does not use it
#
# -------------------------------------------------------------------------------------------
#
# GoLang cross-compile snippet for Go 1.6+ based loosely on Dave Chaney's cross-compile script:
# http://dave.cheney.net/2012/09/08/an-introduction-to-cross-compilation-with-go
#
# To use:
#
#   $ cd ~/path-to/my-awesome-project
#   $ go-build-releases
#
# Features:
#
#   * Cross-compiles to multiple machine types and architectures.
#   * Uses the current directory name as the output name...
#     * ...unless you supply an source file: $ go-build-all main.go
#   * Windows binaries are named .exe.
#   * ARM v5, v6, v7 and v8 (arm64) support
#
# ARM Support:
#
# You must read https://github.com/golang/go/wiki/GoArm for the specifics of running
# Linux/BSD-style kernels and what kernel modules are needed for the target platform.
# While not needed for cross-compilation of this script, you're users will need to ensure
# the correct modules are included.
#
# Requirements:
#
#   * GoLang 1.6+ (for mips and ppc), 1.5 for non-mips/ppc.
#   * CD to directory of the binary you are compiling. $PWD is used here.
#
# For 1.4 and earlier, see http://dave.cheney.net/2012/09/08/an-introduction-to-cross-compilation-with-go
#

# This PLATFORMS list is refreshed after every major Go release.
# Though more platforms may be supported (freebsd/386), they have been removed
# from the standard ports/downloads and therefore removed from this list.
#
#PLATFORMS="$PLATFORMS darwin/amd64" # amd64 only as of go1.5
PLATFORMS="$PLATFORMS windows/amd64" # arm compilation not available for Windows
#PLATFORMS="$PLATFORMS windows/386" # arm compilation not available for Windows
PLATFORMS="$PLATFORMS linux/amd64" 
PLATFORMS="$PLATFORMS linux/arm64" 
#PLATFORMS="$PLATFORMS linux/386"
#PLATFORMS="$PLATFORMS linux/ppc64"
#PLATFORMS="$PLATFORMS linux/ppc64le"
#PLATFORMS="$PLATFORMS linux/mips64" 
#PLATFORMS="$PLATFORMS linux/mips64le" # experimental in go1.6
#PLATFORMS="$PLATFORMS freebsd/amd64"
#PLATFORMS="$PLATFORMS netbsd/amd64" # amd64 only as of go1.6
#PLATFORMS="$PLATFORMS openbsd/amd64" # amd64 only as of go1.6
#PLATFORMS="$PLATFORMS dragonfly/amd64" # amd64 only as of go1.5
#PLATFORMS="$PLATFORMS plan9/amd64"
#PLATFORMS="$PLATFORMS plan9/386" # as of go1.4
#PLATFORMS="$PLATFORMS solaris/amd64" # as of go1.3

# ARMBUILDS lists the platforms that are currently supported.  
#
#   ARM64 (aka ARMv8) <- only supported on linux and darwin builds (go1.6)
#   ARMv7
#   ARMv6
#   ARMv5
#
# Some words of caution from the master:
#
#   @dfc: you'll have to use gomobile to build for darwin/arm64 [and others]
#   @dfc: that target expects that you're bulding for a mobile phone
#   @dfc: iphone 5 and below, ARMv7, iphone 3 and below ARMv6, iphone 5s and above arm64
# 

# Format for arm builds: OS/ARM version (e.g. linux/7 is the ARMv7 build for linux)
#PLATFORMS_ARM="$PLATFORMS_ARM linux/7"
#PLATFORMS_ARM="$PLATFORMS_ARM linux/6"
#PLATFORMS_ARM="$PLATFORMS_ARM linux/5"
#PLATFORMS_ARM="$PLATFORMS_ARM freebsd/7"
#PLATFORMS_ARM="$PLATFORMS_ARM freebsd/6"
#PLATFORMS_ARM="$PLATFORMS_ARM freebsd/5"
#PLATFORMS_ARM="$PLATFORMS_ARM netbsd/7"
#PLATFORMS_ARM="$PLATFORMS_ARM netbsd/6"
#PLATFORMS_ARM="$PLATFORMS_ARM netbsd/5"

##############################################################
# Shouldn't really need to modify anything below this line.  #
##############################################################

if ! command -v zip &> /dev/null
then
    echo "'zip' program could not be found, please install it"
    exit 1
fi


if [[ `git status --porcelain --untracked-files=no` ]]; then
  echo "Uncommitted changes in the repository. Commit or stash before building releases"
  exit 1
else # lets start building
  type setopt >/dev/null 2>&1

  SCRIPT_NAME=`basename "$0"`
  FAILURES=""
  SOURCE_FILE=`echo $@ | sed 's/\.go//'`
  OTHER_DISTRIBUTION_FILES="*.md"
  CURRENT_DIRECTORY=${PWD##*/}
  OUTPUT=${SOURCE_FILE:-$CURRENT_DIRECTORY} # if no src file given, use current dir name
  NOW=$(date +'%Y-%m-%d_%T')
  CTIMESTAMP=$(git show -s --format=%ci HEAD)
  HASH=$(git rev-parse --short HEAD)
  LD_FLAGS=$(echo "-X 'main.sha1ver=$HASH' -X 'main.buildTime=$CTIMESTAMP'")

  for PLATFORM in $PLATFORMS; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    BASE_FILENAME="${OUTPUT}-${GOOS}-${GOARCH}"
    BIN_FILENAME="${BASE_FILENAME}"
    if [[ "${GOOS}" == "windows" ]]; then BIN_FILENAME="${BIN_FILENAME}.exe"; fi
    CMD="GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags=\"${LD_FLAGS}\" -o ${BIN_FILENAME} $@"
    echo "${CMD}"
    eval $CMD || FAILURES="${FAILURES} ${PLATFORM}"
    zip -m ${BASE_FILENAME} ${BIN_FILENAME}
    zip ${BASE_FILENAME}.zip ${OTHER_DISTRIBUTION_FILES}
  done

  # ARM builds
  for PLATFORM_ARM in $PLATFORMS_ARM; do
    GOOS=${PLATFORM_ARM%/*}
    GOARCH="arm"
    GOARM=${PLATFORM_ARM#*/}
    BASE_FILENAME="${OUTPUT}-${GOOS}-${GOARCH}${GOARM}"
    BIN_FILENAME="${BASE_FILENAME}"
    CMD="GOOS=${GOOS} GOARCH=${GOARCH} GOARM=${GOARM} go build -ldflags=\"${LD_FLAGS}\" -o ${BIN_FILENAME} $@"
    echo "${CMD}"
    eval $CMD || FAILURES="${FAILURES} ${PLATFORM}"
    zip -m ${BASE_FILENAME} ${BIN_FILENAME}
    zip ${BASE_FILENAME}.zip ${OTHER_DISTRIBUTION_FILES}
  done

  # eval errors
  if [[ "${FAILURES}" != "" ]]; then
    echo ""
    echo "${SCRIPT_NAME} failed on: ${FAILURES}"
    exit 1
  fi

fi