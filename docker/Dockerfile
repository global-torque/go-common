FROM public.ecr.aws/docker/library/golang:1.25.8-alpine3.23

ARG GIT_COMMIT=unspecified
ARG BUILD_DATE=unspecified
ARG SERVICE_NAME=unspecified
ARG REPOSITORY=unspecified
ARG VERSION
ARG GOLANGCI_LINT_VERSION=v2.11.4
ARG GOSEC_VERSION=v2.22.10
ARG GCI_VERSION=v0.13.7

LABEL GIT_COMMIT=$GIT_COMMIT
LABEL BUILD_DATE=$BUILD_DATE
LABEL SERVICE_NAME=$SERVICE_NAME
LABEL REPOSITORY=$REPOSITORY

ENV GOPATH=/go
ENV VERSION=${VERSION:-unknown}

RUN apk add --no-cache make gcc musl-dev linux-headers git gettext ca-certificates

WORKDIR /app

COPY go.work go.work.sum ./
COPY configurator/go.mod configurator/go.sum ./configurator/
COPY context/go.mod context/go.sum ./context/
COPY db/go.mod db/go.sum ./db/
COPY docker/go.mod docker/go.sum ./docker/
COPY httputils/go.mod httputils/go.sum ./httputils/
COPY logger/go.mod logger/go.sum ./logger/
COPY logger/example/go.mod logger/example/go.sum ./logger/example/
COPY misc/go.mod misc/go.sum ./misc/
COPY queue/go.mod queue/go.sum ./queue/
COPY response/go.mod response/go.sum ./response/
COPY server/go.mod server/go.sum ./server/
COPY tests/go.mod tests/go.sum ./tests/
COPY validator/go.mod validator/go.sum ./validator/
COPY verser/go.mod verser/go.sum ./verser/
RUN go mod download

COPY configurator ./configurator
COPY context ./context
COPY db ./db
COPY docker/main.go ./docker/main.go
COPY httputils ./httputils
COPY logger ./logger
COPY misc ./misc
COPY queue ./queue
COPY response ./response
COPY server ./server
COPY tests ./tests
COPY validator ./validator
COPY verser ./verser

WORKDIR /app/docker
RUN go build -o /app/go-common main.go
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}

RUN go install github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}
RUN go install github.com/daixiang0/gci@${GCI_VERSION}
RUN ln -s /usr/local/go/bin/go /usr/local/bin/go

WORKDIR /app
COPY docker/etc ./etc
RUN addgroup -S app && adduser -S -G app app
USER app
CMD ["/bin/sh"]
