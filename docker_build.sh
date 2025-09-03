#!/bin/bash

# Exit on error
set -xefu

# Configuration
IMAGE_NAME="http-file-server"
IMAGE_TAG=${1:-"latest"}  # Use first argument as tag or default to "latest"
DOCKERFILE_PATH="./Dockerfile"

echo "===== Building Docker Image: ${IMAGE_NAME}:${IMAGE_TAG} ====="

# Build the Docker image
docker build \
  --file ${DOCKERFILE_PATH} \
  --tag ${IMAGE_NAME}:${IMAGE_TAG} \
  --no-cache \
  --progress=plain \
  .

# Show results
echo "===== Build Complete ====="
echo "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
docker images | grep ${IMAGE_NAME}

cat <<EOT
To run the container serving the files in /hostDir/With/Files:
  chmod a+rwx /hostDir/With/Files
  docker run -v /hostDir/With/Files:/DirToServe -p 8080:8080 ${IMAGE_NAME}:${IMAGE_TAG}


EOT