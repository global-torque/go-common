#!/usr/bin/env sh
set -xe

# system functions
basename() {
    # Usage: basename "path" ["suffix"]
    local tmp
    tmp=${1%"${1##*[!/]}"}
    tmp=${tmp##*/}
    tmp=${tmp%'${2/"$tmp"}'}
    printf '%s\n' "${tmp:-/}"
}

lstrip() {
    # Usage: lstrip "string" "pattern"
    printf '%s\n' "${1##$2}"
}

WORK_DIR=$(pwd)
GOLANGCI_LINT_VERSION=v2.11.4
# if company name not set - try to get it from the path$
if [ -z "${COMPANY_NAME}" ]; then
  COMPANY_NAME=$(lstrip $(basename `cd ..; pwd`) "pro")
else
  COMPANY_NAME="${COMPANY_NAME}"
fi

SERVICE_NAME=$(lstrip $(basename $(pwd)) "i-")
REPOSITORY=$COMPANY_NAME/$SERVICE_NAME

# FIX SED for macos
SED=`which gsed` || true
if [ -z "$SED" ]; then
  SED=`which sed`
fi

init() {
  GO_FILES=$(find . -name '*.go' | grep -v _test.go)
  PKG_LIST=$(go list ./... | grep -v /lib/)
}

module_dirs() {
  find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sort
}

build() {
  GIT_COMMIT=$(git rev-parse --short HEAD)
  BUILD_DATE=$(date "+%Y%m%d")

  for file in cmd/*; do
    if [ -d "$file" ]; then
      name=${file##*/}
      go build -ldflags "-s -w -X main.repository=${REPOSITORY} -X main.revisionID=${GIT_COMMIT} -X main.version=${BUILD_DATE}:${GIT_COMMIT} -X main.service=${SERVICE_NAME}" -o ./$name ./cmd/$name/*.go && chmod +x ./$name
    fi
  done
}

self_update() {
  [ ! -d "etc/" ] && mkdir etc;
  docker pull cr.webdevelop.us/webdevelop-pro/go-common:latest-dev;
  docker rm -f makesh;
  docker run --name=makesh cr.webdevelop.us/webdevelop-pro/go-common:latest-dev sh &&
  # docker cp makesh:/app/etc/make.sh make.sh;
  docker cp makesh:/app/etc/golangci.yml .golangci.yml;
  docker cp makesh:/app/etc/air.toml .air.toml
  docker cp makesh:/app/etc/pre-commit etc/pre-commit;
  docker stop makesh;
}

case $1 in

install)
  echo "golang global dependencies"
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin $GOLANGCI_LINT_VERSION
  go install github.com/go-swagger/go-swagger/cmd/swagger@latest
  go install github.com/securego/gosec/v2/cmd/gosec@latest
  go install github.com/cosmtrek/air@latest

  echo "set up pre-commit hook and make.sh file"
  self_update;

  if [ -d ".git" -a -d ".git/hooks" ]
  then
    rm .git/hooks/pre-commit 2>/dev/null || echo 'ok' # ignore error to let sh continue
    ln -s ../../etc/pre-commit .git/hooks/pre-commit
  fi
  ;;

lint)
  shift
  module_dirs | while IFS= read -r ddir; do
    (cd "$ddir" && golangci-lint -c "$WORK_DIR/.golangci.yml" run "$@")
  done
  ;;

vet)
  shift
  module_dirs | while IFS= read -r ddir; do
    (cd "$ddir" && go vet ./... "$@")
  done
  ;;

test)
  shift
  module_dirs | while IFS= read -r ddir; do
    (
      cd "$ddir"
      if [ "${MAKE_LOAD_ENV:-false}" = "true" ] && [ -f ".env" ]; then
        set -a
        . ./.env
        set +a
      fi
      go test -count=1 -p 1 ./... "$@"
    )
  done
  ;;

race)
  init
  go test -race -short ${PKG_LIST}
  ;;

self-update)
  docker rm -f makesh;
  self_update;
  ;;

run-dev)
  # make sure you have proper .air.toml
  air
  ;;

memory)
  init
  CC=clang go test -msan -short ${PKG_LIST}
  ;;

coverage)
  init
  mkdir /tmp/coverage >/dev/null
  rm /tmp/coverage/*.cov
  for package in ${PKG_LIST}; do
    go test -covermode=count -coverprofile "/tmp/coverage/${package##*/}.cov" "$package"
  done
  tail -q -n +2 /tmp/coverage/*.cov >>/tmp/coverage/coverage.cov
  go tool cover -func=/tmp/coverage/coverage.cov
  ;;

run)
  if [ -z "$2" ]
  then
    (build || echo Failed build http) && $(dirname $0)/http
  else
    (build || echo Failed build $2) && $(dirname $0)/$2
  fi

  ;;

audit)
  echo "running gosec"
  gosec ./...
  ;;

build)
  build
  ;;

swag-doc)
  # docs - https://goswagger.io/use/spec/route.html
  swagger generate spec --scan-models -o swagger.json
  ;;

deploy-dev)
  BRANCH_NAME=`git rev-parse --abbrev-ref HEAD`
  GIT_COMMIT=`git rev-parse --short HEAD`
  echo $BRANCH_NAME, $GIT_COMMIT
  docker build -t cr.webdevelop.us/$COMPANY_NAME/$SERVICE_NAME:$GIT_COMMIT -t cr.webdevelop.us/$COMPANY_NAME/$SERVICE_NAME:latest-dev --platform=linux/amd64 .
  # snyk container test cr.webdevelop.us/$COMPANY_NAME/$SERVICE_NAME:$GIT_COMMIT
  if [ $? -ne 0 ]; then
    echo "===================="
    echo "snyk has found a vulnerabilities, please consider choosing alternative image from snyk"
    echo "===================="
  fi
  docker push cr.webdevelop.us/$COMPANY_NAME/$SERVICE_NAME:$GIT_COMMIT
  docker push cr.webdevelop.us/$COMPANY_NAME/$SERVICE_NAME:latest-dev
  kubectl -n $COMPANY_NAME-dev set image deployment/$SERVICE_NAME $SERVICE_NAME=cr.webdevelop.us/$COMPANY_NAME/$SERVICE_NAME:$GIT_COMMIT
  ;;

update-version)
  find ./ -name "go.mod" -exec $SED -i "s/s/(go-common.*) $2/\1 $3/g" {} \;
  module_dirs | while IFS= read -r ddir; do
    (cd "$ddir" && rm -f go.sum && go mod tidy)
  done
  ;;

release-all-pkgs)
  module_dirs | while IFS= read -r ddir; do
    ddir=${ddir#./}
    # echo "$ddir/$2 $3 $4"
    # version and comment
    git tag -a $ddir/$2 $3 "$4"
    git push origin $ddir/$2
  done
  ;;

help)
  cat make.sh | grep "^[a-z-]*)"
  ;;

*)
  echo "unknown $1, try help"
  ;;

esac
