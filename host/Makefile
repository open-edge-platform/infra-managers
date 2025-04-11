# Host Manager Makefile

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

.DEFAULT_GOAL := help
.PHONY: test lint license generate help

all: generate build lint test

# Optionally include tool version checks, not used in Docker builds
TOOL_VERSION_CHECK ?= 1

##### Variables #####

# Required Go Dependencies
GOLINT               := true
GOJUNITREPORT        := true
GOCOBERTURA          := true
PROTOCGENDOC         := true
BUF                  := true
PROTOCGENGO          := true
PROTOCGENGOGRPC      := true
PROTOCGENVALIDATEGO  := true
GNOSTIC              := true
GNOSTICGRPC          := true
PROTOCGENGRPCGATEWAY := true

# Project variables
PROJECT_NAME         := hostmgr
PROJECT_NICKNAME     := h
BINARY_NAME          := $(PROJECT_NAME)

# Code versions, tags, and so on
VERSION              := $(shell cat VERSION)
VERSION_MAJOR        := $(shell cut -c 1 VERSION)
DOCKER_IMG_NAME      := $(PROJECT_NAME)
DOCKER_VERSION       ?= $(shell git branch --show-current | sed 's/\//_/g')
GIT_COMMIT           ?= $(shell git rev-parse HEAD)
GIT_TAG_PREFIX       := host/v

# Test variables
# Set TEST_TARGET to '<testname1>' or '<testname1\|testname2>' to run specific tests or use regex '<testname.*>' - example:
#  TEST_TARGET='TestAuth.*' make test
TEST_TARGET          ?= .
# Default TEST_PKG, overwrite to select particular test pakages
TEST_PKG             ?= $(shell go list ./... | grep -v "pkg/api" | grep -v "/test" | grep -v "/cmd" | tr '\n' ,)
TEST_COVER           := atomic $(shell go list ./... | grep -v "/test" | grep -v "/pkg/api" | grep -v "/cmd")
TEST_USE_DB          := true

# Directory to clean specific to this project
DIR_TO_CLEAN 		 := $(APIPKG_DIR)/hostmgr/proto/*.go

# Include shared makefile
include ../common.mk

generate: buf-gen ## Generate all the code

lint: generate license yamllint go-lint hadolint buf-lint mdlint ## Run all linters and generate targets

test: go-test ## Run all tests

#### Sub-targets ####

$(OUT_DIR)/$(BINARY_NAME): $(SRC) $(DEPS) ## Build if sources or dependencies changed
	$(GOCMD) fmt ./...
	$(GOCMD) build $(GOEXTRAFLAGS) -o $(OUT_DIR)/$(BINARY_NAME) cmd/$(BINARY_NAME)/main.go

go-run: build ## Run go run
	$(GOCMD) run $(GOEXTRAFLAGS) cmd/hostmgr/main.go

buf-update: common-buf-update ## Update buf modules

buf-gen: common-buf-gen ## Compile protoc files into code

buf-lint: common-buf-lint  ## Lint and format protobuf files

#### Integration test targets ####

integration-infra-start:
	$(MAKE) db-stop # additional check just in case $(DB_CONTAINER_NAME) container is still running
	$(DOCKER_COMPOSE) kill && $(DOCKER_COMPOSE) rm -f
	$(DOCKER_COMPOSE) up -d

integration-infra-stop:
	$(DOCKER_COMPOSE) kill && $(DOCKER_COMPOSE) rm -f

test-integration:
	$(GOCMD) test -race -p 1 -failfast -count=1 -v ./test/...
