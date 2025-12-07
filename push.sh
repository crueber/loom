#!/bin/bash
set -e

# -------------------------------------------------------------------
# CONFIGURATION
# -------------------------------------------------------------------
REGISTRY="git.packden.us"
OWNER="crueber"
IMAGE_NAME="loom"
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

TAG_SHA="$REGISTRY/$OWNER/$IMAGE_NAME:$SHA"
TAG_LATEST="$REGISTRY/$OWNER/$IMAGE_NAME:latest"
echo "Building and pushing $TAG_SHA and $TAG_LATEST for $PLATFORM ..."
docker buildx build \
    --platform "$PLATFORM" \
    -t "$TAG_SHA" \
    -t "$TAG_LATEST" \
    --push .
  
echo -e "Successfully built and pushed \n$TAG_SHA and \n$TAG_LATEST"