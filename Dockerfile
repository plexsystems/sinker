FROM golang:1.15.0 AS builder

WORKDIR /build
COPY . /build

ENV CGO_ENABLED=0

RUN go build

FROM alpine:3.12.0

RUN apk update && apk add --no-cache docker-cli

COPY --from=builder /build/sinker /usr/bin/

LABEL org.opencontainers.image.source="https://github.com/plexsystems/sinker"
LABEL org.opencontainers.image.title="sinker"
LABEL org.opencontainers.image.authors="John Reese <john@reese.dev>"
LABEL org.opencontainers.image.description="Sync container images from one registry to another"

ENTRYPOINT ["/usr/bin/sinker"]
