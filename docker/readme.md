# docker

Module path: `github.com/global-torque/go-common/docker`

Build seed for the shared go-common Docker image. This directory is not a
reusable go-common library package.

## Use For

- Building `cr.webdevelop.pro/global-torque/go-common` images.
- Pre-downloading heavy Go dependencies into a shared builder image.
- Shipping common `etc` files such as `make.sh`, `golangci.yml`, `air.toml`,
  and `pre-commit`.

## Do Not Use For

- Service package imports.
- Application runtime logic.

## Key Files

- `docker/Dockerfile`
- `docker/main.go`
- `docker/build-deploy.sh`
- `docker/etc/make.sh`
- `docker/etc/golangci.yml`
- `docker/etc/air.toml`
- `docker/etc/pre-commit`

## Build Configuration

Docker build args:

- `GIT_COMMIT`
- `BUILD_DATE`
- `SERVICE_NAME`
- `REPOSITORY`
- `VERSION`
- `GOLANGCI_LINT_VERSION`
- `GOSEC_VERSION`
- `GCI_VERSION`

The image currently uses Go `1.25.8` on Alpine and installs `golangci-lint`,
`gosec`, and `gci`.

## Wiring Pattern

Dependent service Dockerfiles can use the published image as a builder:

```Dockerfile
FROM cr.webdevelop.us/global-torque/go-common:latest-dev AS builder
RUN ./make.sh build
```

## CI

Root GitHub Actions run vet/tests with PostgreSQL and Pub/Sub emulator services,
then build and push the Docker image.

## Gotchas

- `docker/go.mod` module path is currently `github.com/global-torque/go-common/docker`.
- `docker/main.go` blank-imports common dependencies to warm the image cache.
- This module is a build artifact, not a public package API.
