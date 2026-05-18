# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.26.3-bookworm@sha256:252599aeb51ad60b83e4d8821802068127c528c707cb7dd7afd93be057c6011c AS build

ENV GO111MODULE=on
ARG MAKE_TARGET=go-build
# Arguments used to stamp the binary
ARG REPO_URL
ARG VERSION
ARG REVISION
ARG BUILD_DATE

COPY common.mk version.mk /go/src/github.com/open-edge-platform/infra-managers/
COPY Makefile go.mod go.sum /go/src/github.com/open-edge-platform/infra-managers/networking/
COPY cmd/ /go/src/github.com/open-edge-platform/infra-managers/networking/cmd/
COPY internal/ /go/src/github.com/open-edge-platform/infra-managers/networking/internal/
COPY vendor/ /go/src/github.com/open-edge-platform/infra-managers/networking/vendor/

WORKDIR /go/src/github.com/open-edge-platform/infra-managers/networking

RUN CGO_ENABLED=0 LABEL_REPO_URL=${REPO_URL} LABEL_VERSION=${VERSION} LABEL_REVISION=${REVISION} LABEL_BUILD_DATE=${BUILD_DATE} make ${MAKE_TARGET}

FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
# Run as non-privileged user
USER nobody

ARG REPO_URL
ARG VERSION
ARG REVISION
ARG BUILD_DATE

LABEL org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.source=${REPO_URL} \
      org.opencontainers.image.revision=${REVISION} \
      org.opencontainers.image.created=${BUILD_DATE}

COPY --from=build --chown=nobody:nobody /go/src/github.com/open-edge-platform/infra-managers/networking/out/netmgr /usr/local/bin/netmgr

ENTRYPOINT ["netmgr"]
