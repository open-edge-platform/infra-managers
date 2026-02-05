# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.25.5-bookworm as build

ENV GO111MODULE=on
ARG MAKE_TARGET=go-build
# Arguments used to stamp the binary
ARG REPO_URL
ARG VERSION
ARG REVISION
ARG BUILD_DATE

COPY common.mk version.mk /go/src/github.com/open-edge-platform/infra-managers/
COPY Makefile go.mod go.sum /go/src/github.com/open-edge-platform/infra-managers/telemetry/
COPY cmd/ /go/src/github.com/open-edge-platform/infra-managers/telemetry/cmd/
COPY vendor/ /go/src/github.com/open-edge-platform/infra-managers/telemetry/vendor/
COPY internal/ /go/src/github.com/open-edge-platform/infra-managers/telemetry/internal/
COPY pkg/ /go/src/github.com/open-edge-platform/infra-managers/telemetry/pkg/
COPY rego/ /go/src/github.com/open-edge-platform/infra-managers/telemetry/rego/

WORKDIR /go/src/github.com/open-edge-platform/infra-managers/telemetry

RUN CGO_ENABLED=0 LABEL_REPO_URL=${REPO_URL} LABEL_VERSION=${VERSION} LABEL_REVISION=${REVISION} LABEL_BUILD_DATE=${BUILD_DATE} make ${MAKE_TARGET}

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

COPY --from=build --chown=nobody:nobody /go/src/github.com/open-edge-platform/infra-managers/telemetry/out/telemetrymgr /usr/local/bin/telemetrymgr
# Copy policy bundle
COPY --from=build --chown=nobody:nobody /go/src/github.com/open-edge-platform/infra-managers/telemetry/rego/authz.rego /rego/authz.rego

ENTRYPOINT ["telemetrymgr"]
