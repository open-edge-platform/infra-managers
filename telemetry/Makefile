# Telemetry Manager Makefile

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
PROTOCGENGO          := true
PROTOCGENGOGRPC      := true
PROTOCGENVALIDATEGO  := true

# Project variables
PROJECT_NAME         := telemetrymgr
PROJECT_NICKNAME     := t
BINARY_NAME          := $(PROJECT_NAME)
SUBPROJECT_DIR       := telemetry

# Code Versions
VERSION              := $(shell cat VERSION)
VERSION_MAJOR        := $(shell cut -c 1 VERSION)
DOCKER_IMG_NAME      := $(PROJECT_NAME)
DOCKER_VERSION       ?= $(shell git branch --show-current | sed 's/\//_/g')
GIT_COMMIT           ?= $(shell git rev-parse HEAD)
GIT_TAG_PREFIX       := telemetry/v

# Test variables
# Set TEST_TARGET to '<testname1>' or '<testname1\|testname2>' to run specific tests or use regex '<testname.*>' - example:
#  TEST_TARGET='TestAuth.*' make test
TEST_TARGET         ?= .
# Default TEST_PKG, overwrite to select particular test pakages
TEST_PKG            ?= $(shell go list ./internal/... | tr '\n' ,)
TEST_COVER          := atomic ./...
TEST_ARGS           := -globalLogLevel=error
TEST_USE_DB         := true

# Directory to clean specific to this project
DIR_TO_CLEAN        :=

# Include shared makefile
include ../common.mk

generate: buf-gen

lint: generate license yamllint go-lint hadolint buf-lint mdlint ## Run all linters

test: go-test ## Run all tests

#### Sub-targets ####

$(OUT_DIR)/$(BINARY_NAME): $(SRC) $(DEPS) ## Build if sources or dependencies changed
	$(GOCMD) build $(GOEXTRAFLAGS) -o $(OUT_DIR)/$(BINARY_NAME) cmd/$(BINARY_NAME)/main.go

go-run: build ## go run
	$(GOCMD) run $(GOEXTRAFLAGS) cmd/$(BINARY_NAME)/$(BINARY_NAME).go --rbacRules $(RBAC)

buf-update: common-buf-update ## Update buf modules

buf-gen: common-buf-gen ## Compile protoc files into code

buf-lint: ## Lint and format protobuf files
	buf --version
	# TODO: currently failing lint, need to fix
	#buf format -d --exit-code
	#buf lint
	# TODO: disabling breaking check, re-enable when breaking change is merged
