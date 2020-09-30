ARG GOLANG_VERSION=1.15.0
ARG ALPINE_VERSION=3.12.0

FROM golang:${GOLANG_VERSION} AS builder
WORKDIR /build
COPY . /build

ENV CGO_ENABLED=0

RUN go get && \
    go build

FROM alpine:${ALPINE_VERSION}

# OCI annotations (https://github.com/opencontainers/image-spec/blob/master/annotations.md)
LABEL org.opencontainers.image.source="https://github.com/plexsystems/sinker" \
    org.opencontainers.image.title="sinker" \
    org.opencontainers.image.authors="John Reese <john@reese.dev>" \
    org.opencontainers.image.description="Application to sync images from one registry to another"

# explicitly set user/group IDs
# RUN set -eux \
#     && addgroup -g 1001 -S sinker \
#     && adduser -S -D -H -u 1001 -s /sbin/nologin -G sinker -g sinker sinker

RUN apk update && apk add --no-cache docker-cli

COPY --from=builder /build/sinker /usr/bin/

# USER sinker

ENTRYPOINT ["/usr/bin/sinker"]
