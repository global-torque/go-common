#!/bin/sh

set -eu

COMPANY_NAME="${COMPANY_NAME:-global-torque}"
SERVICE_NAME="${SERVICE_NAME:-go-common}"
REGISTRY="${REGISTRY:-cr.webdevelop.pro}"
CONTAINER_ENGINE="${CONTAINER_ENGINE:-docker}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPOSITORY_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

if [ -n "$(git -C "$REPOSITORY_ROOT" status --porcelain --untracked-files=all)" ]; then
  echo "Refusing to publish from a dirty worktree." >&2
  exit 1
fi

FULL_GIT_COMMIT=$(git -C "$REPOSITORY_ROOT" rev-parse HEAD)
SHORT_GIT_COMMIT=$(printf '%s' "$FULL_GIT_COMMIT" | cut -c1-8)
BUILD_DATE=$(date -u "+%Y%m%d")
VERSION="$BUILD_DATE:$FULL_GIT_COMMIT"
REPOSITORY="$COMPANY_NAME/$SERVICE_NAME"
IMAGE="$REGISTRY/$REPOSITORY"
TMPDIR=${TMPDIR:-/tmp/opencode}
mkdir -p "$TMPDIR"
METADATA_FILE=$(mktemp "$TMPDIR/go-common-build-metadata.XXXXXX")
IMMUTABLE_DIGEST_FILE=$(mktemp "$TMPDIR/go-common-immutable-digest.XXXXXX")
CHANNEL_DIGEST_FILE=$(mktemp "$TMPDIR/go-common-channel-digest.XXXXXX")
LOCAL_MANIFEST=""

cleanup() {
  if [ -n "$LOCAL_MANIFEST" ]; then
    "$CONTAINER_ENGINE" manifest rm "$LOCAL_MANIFEST" >/dev/null 2>&1 || true
  fi
  rm -f "$METADATA_FILE" "$IMMUTABLE_DIGEST_FILE" "$CHANNEL_DIGEST_FILE"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

require_digest() {
  digest=$1
  label=$2
  if ! printf '%s\n' "$digest" | grep -Eq '^sha256:[0-9a-f]{64}$'; then
    echo "$label did not produce a valid image digest: $digest" >&2
    exit 1
  fi
}

build_with_docker() {
  echo "Building and pushing $IMAGE for $PLATFORMS with Docker Buildx"
  "$CONTAINER_ENGINE" buildx build \
    --file "$SCRIPT_DIR/Dockerfile" \
    --platform "$PLATFORMS" \
    --build-arg GIT_COMMIT="$FULL_GIT_COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    --build-arg SERVICE_NAME="$SERVICE_NAME" \
    --build-arg REPOSITORY="$REPOSITORY" \
    --build-arg VERSION="$VERSION" \
    --tag "$IMAGE:$SHORT_GIT_COMMIT" \
    --tag "$IMAGE:latest-dev" \
    --metadata-file "$METADATA_FILE" \
    --push \
    "$REPOSITORY_ROOT"

  image_digest=$(sed -n 's/.*"containerimage.digest":[[:space:]]*"\(sha256:[0-9a-f]*\)".*/\1/p' "$METADATA_FILE" | head -n 1)
  require_digest "$image_digest" "Docker Buildx"
  printf 'image_digest=%s\n' "$image_digest"
}

build_with_podman() {
  LOCAL_MANIFEST="$SERVICE_NAME-$SHORT_GIT_COMMIT-multiarch"

  echo "Building $LOCAL_MANIFEST for $PLATFORMS with Podman"
  "$CONTAINER_ENGINE" manifest rm "$LOCAL_MANIFEST" >/dev/null 2>&1 || true
  "$CONTAINER_ENGINE" build \
    --file "$SCRIPT_DIR/Dockerfile" \
    --platform "$PLATFORMS" \
    --manifest "$LOCAL_MANIFEST" \
    --build-arg GIT_COMMIT="$FULL_GIT_COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    --build-arg SERVICE_NAME="$SERVICE_NAME" \
    --build-arg REPOSITORY="$REPOSITORY" \
    --build-arg VERSION="$VERSION" \
    "$REPOSITORY_ROOT"

  echo "Pushing $IMAGE:$SHORT_GIT_COMMIT"
  "$CONTAINER_ENGINE" manifest push --all --digestfile "$IMMUTABLE_DIGEST_FILE" \
    "$LOCAL_MANIFEST" "docker://$IMAGE:$SHORT_GIT_COMMIT"

  echo "Pushing $IMAGE:latest-dev"
  "$CONTAINER_ENGINE" manifest push --all --digestfile "$CHANNEL_DIGEST_FILE" \
    "$LOCAL_MANIFEST" "docker://$IMAGE:latest-dev"

  immutable_digest=$(cat "$IMMUTABLE_DIGEST_FILE")
  channel_digest=$(cat "$CHANNEL_DIGEST_FILE")
  require_digest "$immutable_digest" "Podman immutable push"
  require_digest "$channel_digest" "Podman latest-dev push"
  if [ "$immutable_digest" != "$channel_digest" ]; then
    echo "Podman pushes produced different digests: $immutable_digest != $channel_digest" >&2
    exit 1
  fi
  printf 'image_digest=%s\n' "$immutable_digest"
}

case "$(basename "$CONTAINER_ENGINE")" in
  podman)
    build_with_podman
    ;;
  *)
    BUILDX_VERSION=$("$CONTAINER_ENGINE" buildx version 2>/dev/null || true)
    case "$BUILDX_VERSION" in
      *github.com/docker/buildx*) build_with_docker ;;
      *buildah* | *Buildah*) build_with_podman ;;
      *)
        echo "A Docker Buildx or Podman-compatible container engine is required." >&2
        exit 1
        ;;
    esac
    ;;
esac
