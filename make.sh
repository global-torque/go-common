#!/usr/bin/env sh
# Canonical make.sh for Go API services.
# Source of truth lives at ansible-devops/templates/make.sh/go.sh.
# Re-sync all repos with: tools/sync-makesh.sh (TODO).
#
# Usage:
#   ./make.sh build [NAME]       # compile binaries under cmd/*
#   ./make.sh run [ARGS...]      # execute the built ./http binary
#   ./make.sh run-dev [http|cloudrun]
#   ./make.sh test               # go test -count=1 ./...
#   ./make.sh lint               # golangci-lint
#   ./make.sh swagger             # generate swagger.yaml
#   ./make.sh swagger-run         # generate swagger.yaml and preview on http://localhost:8081
#   ./make.sh publish dev|prod   # build once and publish image aliases
#   ./make.sh deploy-worker dev|prod # publish and update the mapped Cloud Run worker
#   ./make.sh help                # list commands
# ShellCheck: this template uses the `local` extension provided by the
# repository's sh-compatible shells, and keeps selected word splitting for
# existing Go/build-compose argument contracts.
# shellcheck disable=SC3043
set -eu

# Optional per-repo overrides are for service identity and other local build
# settings. A worker is enabled only when WORKER_IMAGE is explicitly set.
# shellcheck disable=SC1091
[ -f "$(pwd)/.makerc" ] && . "$(pwd)/.makerc"

# ----- Identity ---------------------------------------------------------------
WORK_DIR=$(pwd)
SERVICE_NAME=${SERVICE_NAME:-$(basename "$WORK_DIR" | sed 's/^i-//')}
COMPANY_NAME=${COMPANY_NAME:-torque-investments}
REGISTRY=${REGISTRY:-cr.webdevelop.pro}
REPO_IMAGE=${REPO_IMAGE:-$REGISTRY/$COMPANY_NAME/$SERVICE_NAME}

# GIT_COMMIT is supplied by CI when available and otherwise comes from the
# checked-out revision.
CURRENT_HEAD=$(git rev-parse --verify HEAD 2>/dev/null || true)
GIT_COMMIT=${GIT_COMMIT:-$CURRENT_HEAD}
BUILD_DATE=${BUILD_DATE:-$(git show -s --format=%cd --date=format:%Y%m%d "$GIT_COMMIT" 2>/dev/null || date -u +%Y%m%d)}
WORKER_NAME=${WORKER_NAME:-${SERVICE_NAME%-api}-worker}

PKG_LIST() { go list -buildvcs=false ./... | grep -v /lib/; }

# ----- Build ------------------------------------------------------------------
build() {
  local only="${1:-}"
  local tag_args=""
  [ -n "${GO_BUILD_TAGS:-}" ] && tag_args="-tags=${GO_BUILD_TAGS}"
  for d in cmd/*; do
    [ -d "$d" ] || continue
    bin=${d##*/}
    [ -n "$only" ] && [ "$bin" != "$only" ] && continue
    binary_service_name=$SERVICE_NAME
    case "$bin" in
      worker|cloudrun) binary_service_name=$WORKER_NAME ;;
    esac
    # shellcheck disable=SC2086
    go build \
      $tag_args \
      -ldflags "-s -w -X main.repository=$COMPANY_NAME/$SERVICE_NAME -X main.revisionID=$GIT_COMMIT -X main.version=$BUILD_DATE:$GIT_COMMIT -X main.service=$binary_service_name" \
      -o "./$bin" ./cmd/"$bin"/*.go
    chmod +x "./$bin"
  done
}

# ----- Publication ------------------------------------------------------------
publish() {
  env=${1:-}
  case "$env" in
    dev|prod) ;;
    *) echo "usage: ./make.sh publish dev|prod" >&2; exit 2 ;;
  esac
  api_ref="$REPO_IMAGE:$GIT_COMMIT"
  api_channel="$REPO_IMAGE:latest-$env"
  worker_ref=""
  if [ -n "${WORKER_IMAGE:-}" ]; then
    worker_ref="$WORKER_IMAGE:$GIT_COMMIT"
    worker_channel="$WORKER_IMAGE:latest-$env"
  fi

  # Build once. The same local image is reused for the API and optional worker.
  docker build \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    --build-arg SERVICE_NAME="$SERVICE_NAME" \
    --build-arg REPOSITORY="$COMPANY_NAME/$SERVICE_NAME" \
    --build-arg GO_BUILD_TAGS="${GO_BUILD_TAGS:-}" \
    --platform=linux/amd64 \
    -t "$api_ref" \
    . >&2

  docker push "$api_ref" >&2
  docker tag "$api_ref" "$api_channel" >&2
  docker push "$api_channel" >&2

  if [ -n "${WORKER_IMAGE:-}" ]; then
    docker tag "$api_ref" "$worker_ref" >&2
    docker push "$worker_ref" >&2
    docker tag "$worker_ref" "$worker_channel" >&2
    docker push "$worker_channel" >&2
  fi

  printf 'api_image=%s\n' "$api_ref"
  if [ -n "$worker_ref" ]; then
    printf 'worker_image=%s\n' "$worker_ref"
  fi
}

# ----- Cloud Run worker release -----------------------------------------------
deploy_worker() {
  release_env=${1:-}
  case "$release_env" in
    dev)
      service="$WORKER_NAME-dev"
      region=europe-central2
      ;;
    prod)
      service="$WORKER_NAME"
      region=europe-west6
      ;;
    *)
      echo "usage: ./make.sh deploy-worker dev|prod" >&2
      exit 2
      ;;
  esac

  WORKER_IMAGE=europe-west6-docker.pkg.dev/webdevelop-live/torque-investments/$WORKER_NAME
  publish "$release_env"
  gcloud run services update "$service" --project webdevelop-live --region "$region" --container "$service" --image="$WORKER_IMAGE:$GIT_COMMIT"
}

# ----- Run (dev shortcut) -----------------------------------------------------
run() {
  if [ ! -x ./http ]; then
    echo "./http not found or is not executable; run ./make.sh build first" >&2
    exit 2
  fi
  exec ./http "$@"
}

run_dev() {
  local bin="${1:-http}"
  case "$bin" in
    http|cloudrun) ;;
    *) echo "usage: ./make.sh run-dev [http|cloudrun]" >&2; exit 2 ;;
  esac
  if [ ! -d "cmd/$bin" ]; then
    echo "cmd/$bin/ not found in $(pwd)" >&2
    exit 2
  fi
  exec go run ./cmd/"$bin"/
}

run_tests() {
  TZ=UTC go test -count=1 ./...
}

run_lint() {
  golangci-lint -c .golangci.yml run --fix "$@"
}

swagger_file() {
  if [ -n "${SWAGGER_FILE:-}" ]; then
    printf '%s\n' "$SWAGGER_FILE"
  elif [ -f api/swagger.yaml ]; then
    printf '%s\n' "api/swagger.yaml"
  else
    printf '%s\n' "swagger.yaml"
  fi
}

swagger_abs_file() {
  local file
  file=$(swagger_file)
  case "$file" in
    /*) printf '%s\n' "$file" ;;
    *)  printf '%s\n' "$(pwd)/$file" ;;
  esac
}

generate_swagger() {
  local file
  local out_dir
  local swagger_bin

  file=$(swagger_file)
  out_dir=$(dirname "$file")
  mkdir -p "$out_dir"

  swagger_bin=$(command -v swagger 2>/dev/null || true)
  if [ -n "$swagger_bin" ]; then
    "$swagger_bin" generate spec --scan-models -o "$file"
  else
    go run github.com/go-swagger/go-swagger/cmd/swagger@latest generate spec --scan-models -o "$file"
  fi
}

swagger_run() {
  local file

  generate_swagger
  file=$(swagger_abs_file)
  docker run --rm -p 8081:8080 \
    -v "$file:/spec/swagger.yaml:ro" \
    redocly/cli:latest preview-docs /spec/swagger.yaml --host 0.0.0.0 --port 8080
}

print_help() {
  grep '^#   ./make.sh' "$0" | sed 's/^# *//'
  if command -v print_custom_help >/dev/null 2>&1; then
    print_custom_help
  fi
}

# Per-repo function overrides are loaded after canonical functions and before
# dispatch. Vars-only tweaks belong in ./.makerc instead.
# shellcheck disable=SC1091
[ -f "$(pwd)/.make.override.sh" ] && . "$(pwd)/.make.override.sh"

# ----- Commands ---------------------------------------------------------------
case ${1:-help} in
  build)       shift; build "${1:-}" ;;
  run)         shift; run "$@" ;;
  build-*)     build "${1#build-}" ;;
  run-dev)     shift; run_dev "$@" ;;
  test)        shift; run_tests "$@" ;;
  lint)        shift; run_lint "$@" ;;
  swagger)     generate_swagger ;;
  swagger-run) swagger_run ;;
  publish)     shift; publish "${1:-}" ;;
  deploy-worker) shift; deploy_worker "${1:-}" ;;
  help)        print_help ;;
  *)
    if command -v custom_command >/dev/null 2>&1; then
      set +e
      custom_command "$@"
      rc=$?
      set -e
      if [ "$rc" -ne 127 ]; then
        exit "$rc"
      fi
    fi
    echo "unknown command: ${1:-}" >&2
    print_help >&2
    exit 2
    ;;
esac
