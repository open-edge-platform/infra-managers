# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.25.5-bookworm as build

SHELL ["/bin/bash", "-euo", "pipefail", "-c"]

ARG MAKE_TARGET=build

# Arguments used to stamp the binary
ARG REPO_URL
ARG VERSION
ARG REVISION
ARG BUILD_DATE

COPY common.mk version.mk /go/src/github.com/open-edge-platform/infra-managers/
COPY Makefile common.mk requirements.txt VERSION go.mod go.sum /go/src/github.com/open-edge-platform/infra-managers/os-resource/
COPY cmd/ /go/src/github.com/open-edge-platform/infra-managers/os-resource/cmd/
COPY internal/ /go/src/github.com/open-edge-platform/infra-managers/os-resource/internal/
COPY vendor/ /go/src/github.com/open-edge-platform/infra-managers/os-resource/vendor/

WORKDIR /go/src/github.com/open-edge-platform/infra-managers/os-resource

RUN CGO_ENABLED=0 TOOL_VERSION_CHECK=0 \
  LABEL_REPO_URL=${REPO_URL} \
  LABEL_VERSION=${VERSION} \
  LABEL_REVISION=${REVISION} \
  LABEL_BUILD_DATE=${BUILD_DATE} \
  make ${MAKE_TARGET}

# From: https://github.com/GoogleContainerTools/distroless
FROM gcr.io/distroless/static-debian12:nonroot

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

# Set default inventory ticker period (can be overridden via environment variable)
ENV INVENTORY_TICKER_PERIOD=12h

COPY --from=build --chown=nobody:nobody /go/src/github.com/open-edge-platform/infra-managers/os-resource/out/osrm /usr/local/bin/osrm

ENTRYPOINT ["osrm"]
