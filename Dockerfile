FROM golang:1.17 AS builder

ENV CGO_ENABLED=0

WORKDIR /build
COPY . /build

# SINKER_VERSION is set during the release process
ARG SINKER_VERSION=0.0.0
RUN go build -tags 'containers_image_openpgp' -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=${SINKER_VERSION}'"

FROM alpine:3.14.6

RUN apk update && apk add --no-cache docker-cli

COPY --from=builder /build/sinker /usr/bin/

LABEL org.opencontainers.image.source="https://github.com/plexsystems/sinker"
LABEL org.opencontainers.image.title="sinker"
LABEL org.opencontainers.image.authors="John Reese <john@reese.dev>"
LABEL org.opencontainers.image.description="Sync container images from one registry to another"

ENTRYPOINT ["/usr/bin/sinker"]
