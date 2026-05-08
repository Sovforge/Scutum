#!/bin/bash
# Build the Scutum container image with SBOM attestation and provenance.
# Usage: ./scripts/build-image.sh [version] [registry/image]
set -euo pipefail

VERSION=${1:-dev}
IMAGE=${2:-scutum}
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
VCS_REF=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Building ${IMAGE}:${VERSION} (ref=${VCS_REF})"

docker buildx build \
  --build-arg VERSION="${VERSION}" \
  --build-arg BUILD_DATE="${BUILD_DATE}" \
  --build-arg VCS_REF="${VCS_REF}" \
  --sbom=true \
  --provenance=true \
  --tag "${IMAGE}:${VERSION}" \
  --tag "${IMAGE}:latest" \
  --file dockerfile.yml \
  --load \
  .

echo "Done. Inspect attestations with:"
echo "  docker buildx imagetools inspect ${IMAGE}:${VERSION}"
echo "In-image SBOM is at /var/lib/sbom/sbom.spdx.json"
