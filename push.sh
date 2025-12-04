#!/bin/bash
set -e

# -------------------------------------------------------------------
# CONFIGURATION
# -------------------------------------------------------------------
REGISTRY="git.packden.us"
OWNER="crueber"
IMAGE_NAME="home-links"
PLATFORM="linux/amd64,linux/arm64"

# -------------------------------------------------------------------
# SCRIPT START
# -------------------------------------------------------------------

SHA=$(git rev-parse --short HEAD)
if [ -z "$SHA" ]; then
  echo "Error: could not determine git commit SHA."
  exit 1
fi

if ! command -v docker &>/dev/null; then
  echo "Error: docker command not found."
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "Error: Docker daemon is not running or you lack permissions."
  exit 1
fi

BUILDER="multiarch-builder"
if ! docker buildx ls | grep -q "$BUILDER"; then
  echo "Creating Buildx builder '$BUILDER'..."
  docker buildx create --name "$BUILDER" --driver docker-container --use
  docker buildx inspect --bootstrap
fi
docker buildx use "$BUILDER"

TAG="$REGISTRY/$OWNER/$IMAGE_NAME:$SHA"
echo "Building and pushing $TAG for $PLATFORM ..."
docker buildx build \
  --platform "$PLATFORM" \
  -t "$TAG" \
  --push .

docker tag "$TAG" "$REGISTRY/$OWNER/$IMAGE_NAME:latest"
docker push "$REGISTRY/$OWNER/$IMAGE_NAME:latest"

echo "Successfully built and pushed $TAG"
