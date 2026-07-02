# Go lang common image

Pre-build docker image for go 1.23.0
- Improve docker build up for 10x times
- Make sure base image is secure with snyk vulnerability scanner
- Universal make.sh file to help have similar pipelines on different go based repos

## Installation

1. Set up credentials for cr.webdevelop.pro and docker.io/global-torque
2. Set up credentials for [snyk](https://snyk.io/) to verify image security vulnerabilities

## Deploy
1. build and deploy using `./build-deploy.sh` script

## Structure
- build all heavy dependencies (gcc, fx.uber, modern-go/concurrent, modern-go/reflect2)
- `etc/golangci.yml` - actuall rules for golang linter
- `etc/make.sh` - bash utility with usefull commands
- `etc/pre-commit` - git pre-commit rules
- `etc/air.toml` - autorestart service on changes


## Usage example
```Dockerfile
FROM cr.webdevelop.us/global-torque/go-common:latest-dev AS builder

# RUN apk add --no-cache make gcc musl-dev linux-headers git gettext - no longer needed
# fast build cause of pre-build requirements
RUN ./make.sh build 
```

# ToDo
- [ ] `./make.sh coverage` to generate badger for test coverage
- [ ] `./make.sh run-debug-dev` add ability to run in debug mode
- [ ] create go image with pubsub and without pubsub (pubsub takes about 1G of space)
