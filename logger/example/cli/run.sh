#!/bin/sh

REPOSITORY="github.com/global-torque/user-api"
GIT_COMMIT="a14625319b802c417e6b89645cd676596c497adc"
VERSION="1.2.3"
SERVICE_NAME="user-cli"
go build -ldflags "-s -w -X main.repository=${REPOSITORY} -X main.revisionID=${GIT_COMMIT} -X main.version=${VERSION}:${GIT_COMMIT} -X main.service=${SERVICE_NAME}" -o ./app ./*.go && chmod +x ./app &&
./app