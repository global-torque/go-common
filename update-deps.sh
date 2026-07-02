#!/bin/bash

set -e

if [ ! -d ".git" ]; then
  echo "Not found .git directory. Please run the script in the root of the repository."
  exit 1
fi

TAG_VERSION="v1.0.19"

# get go.mod from dir in root
MODULE_DIRS=$(find . -maxdepth 2 -name "go.mod" -exec dirname {} \;)

for DIR in $MODULE_DIRS; do
  # check if the dir is one level up from the root

  pushd "$DIR"
  echo "Updating dependencies for module: $DIR"

  grep -E 'github.com/global-torque/go-common/.* v' go.mod | while read -r line; do
    DEP=$(echo $line | awk '{print $1}')
    echo "Updating dependency: $DEP to version $TAG_VERSION"
    go get "$DEP@$TAG_VERSION"
  done

  go mod tidy

  popd
done

echo "All dependencies have been updated to version $TAG_VERSION."
