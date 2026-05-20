#!/bin/bash
set -euo pipefail
#
# Build a Go test fixture binary using the builder container.
#
# Usage:
#   build_fixture.sh [options]
#
# Options:
#   --goarch=ARCH        Target: amd64, s390x, arm64, ppc64le (default: amd64)
#   --buildmode=MODE     Go build mode: exe, pie (default: exe)
#   --cgo                Enable CGO with zig cross-linker
#   --fips               Enable GOFIPS140 (auto-detects in-process version)
#   --go-image=TAG       go-toolset image tag (default: 1.26)
#   --output=NAME        Output binary name (default: auto-generated)
#
# Examples:
#   # Go 1.24 s390x with CGO (external linkage via zig)
#   ./build_fixture.sh --goarch=s390x --cgo --go-image=1.24
#
#   # Go 1.26 s390x PIE with FIPS
#   ./build_fixture.sh --goarch=s390x --cgo --buildmode=pie --fips
#
#   # Go 1.24 amd64 internal PIE (no CGO, no zig needed)
#   ./build_fixture.sh --buildmode=pie --go-image=1.24
#
#   # Go 1.20 arm64 with CGO and FIPS (boringcrypto era)
#   ./build_fixture.sh --goarch=arm64 --cgo --fips --go-image=1.20

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
BUILDER_IMAGE_BASE="check-payload-fixture-builder"

GOARCH="amd64"
BUILDMODE="exe"
CGO=false
FIPS=false
GO_IMAGE_TAG="1.26"
OUTPUT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --goarch=*)    GOARCH="${1#*=}"; shift ;;
    --buildmode=*) BUILDMODE="${1#*=}"; shift ;;
    --cgo)         CGO=true; shift ;;
    --fips)        FIPS=true; shift ;;
    --go-image=*)  GO_IMAGE_TAG="${1#*=}"; shift ;;
    --output=*)    OUTPUT="${1#*=}"; shift ;;
    -h|--help)
      sed -n '3,/^$/s/^# \?//p' "$0"
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

declare -A ZIG_TARGETS=(
  [s390x]="s390x-linux-gnu"
  [arm64]="aarch64-linux-gnu"
  [amd64]="x86_64-linux-gnu"
  [ppc64le]="powerpc64le-linux-gnu"
)

if [[ -z "${ZIG_TARGETS[$GOARCH]+x}" ]]; then
  echo "Unsupported GOARCH: $GOARCH (supported: ${!ZIG_TARGETS[*]})" >&2
  exit 1
fi

if [[ -z "$OUTPUT" ]]; then
  OUTPUT="fixture_go${GO_IMAGE_TAG}_${GOARCH}"
  [[ "$BUILDMODE" != "exe" ]] && OUTPUT="${OUTPUT}_${BUILDMODE}"
  [[ "$FIPS" == true ]] && OUTPUT="${OUTPUT}_fips"
  [[ "$CGO" == true ]] && OUTPUT="${OUTPUT}_cgo"
fi

BUILDER_TAG="${BUILDER_IMAGE_BASE}:go${GO_IMAGE_TAG}"
BASE_IMAGE="registry.access.redhat.com/ubi9/go-toolset:${GO_IMAGE_TAG}"

if ! ${CONTAINER_RUNTIME} image exists "${BUILDER_TAG}" 2>/dev/null; then
  echo "Building container image ${BUILDER_TAG}..."
  ZIG_ARCH=$(uname -m)
  ${CONTAINER_RUNTIME} build \
    --build-arg "BASE_IMAGE=${BASE_IMAGE}" \
    --build-arg "ZIG_ARCH=${ZIG_ARCH}" \
    -t "${BUILDER_TAG}" \
    -f "${SCRIPT_DIR}/Containerfile.builder" \
    "${SCRIPT_DIR}"
fi

BUILD_ENV="GOOS=linux GOARCH=${GOARCH}"

HOST_ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/;s/ppc64le/ppc64le/')
if [[ "$CGO" == true ]]; then
  BUILD_ENV="${BUILD_ENV} CGO_ENABLED=1"
  if [[ "$GOARCH" != "$HOST_ARCH" ]]; then
    BUILD_ENV="${BUILD_ENV} CC=\"zig cc -target ${ZIG_TARGETS[$GOARCH]}\""
  fi
else
  BUILD_ENV="${BUILD_ENV} CGO_ENABLED=0"
fi

# Red Hat Go uses the in-process FIPS module version from
# lib/fips140/inprocess.txt, not the upstream "certified" alias.
if [[ "$FIPS" == true ]]; then
  FIPS_VERSION=$(${CONTAINER_RUNTIME} run --rm "${BUILDER_TAG}" \
    cat /usr/lib/golang/lib/fips140/inprocess.txt 2>/dev/null || echo "certified")
  BUILD_ENV="${BUILD_ENV} GOFIPS140=${FIPS_VERSION}"
fi

BUILD_FLAGS="-buildmode=${BUILDMODE}"
[[ "$BUILDMODE" == "pie" ]] && BUILD_FLAGS="${BUILD_FLAGS} -ldflags=-s\\ -w -trimpath"

CONTAINER_ID=$(${CONTAINER_RUNTIME} create \
  -v "${SCRIPT_DIR}/main.go:/src/main.go:ro,Z" \
  "${BUILDER_TAG}" \
  bash -c "
    cp /src/main.go . && \
    go mod init fixture 2>/dev/null && \
    go mod tidy 2>/dev/null && \
    ${BUILD_ENV} go build -a ${BUILD_FLAGS} -o /build/output .
  ")

${CONTAINER_RUNTIME} start -a "${CONTAINER_ID}"
${CONTAINER_RUNTIME} cp "${CONTAINER_ID}:/build/output" "${SCRIPT_DIR}/${OUTPUT}"
${CONTAINER_RUNTIME} rm -f "${CONTAINER_ID}" >/dev/null 2>&1 || true

echo "Built: ${SCRIPT_DIR}/${OUTPUT}"
echo "  Go image: ${BASE_IMAGE}"
echo "  GOARCH:   ${GOARCH}"
echo "  Mode:     ${BUILDMODE}"
echo "  CGO:      ${CGO}"
echo "  FIPS:     ${FIPS}"
