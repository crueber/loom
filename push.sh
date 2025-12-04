#!/bin/bash
set -e

# -------------------------------------------------------------------
# CONFIGURATION
# -------------------------------------------------------------------
REGISTRY="git.packden.us"
OWNER="crueber"
IMAGE_NAME="home-links"

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

TAG="$REGISTRY/$OWNER/$IMAGE_NAME:$SHA"
echo "Building $TAG ..."
docker build -t "$TAG" .

echo "Pushing $TAG ..."
docker push "$TAG"

docker tag "$TAG" "$REGISTRY/$OWNER/$IMAGE_NAME:latest"
docker push "$REGISTRY/$OWNER/$IMAGE_NAME:latest"

echo "Successfully built and pushed $TAG"
