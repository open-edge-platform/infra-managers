# OS Resource Manager Makefile

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

.DEFAULT_GOAL := help
.PHONY: test lint license generate help

all: generate build lint test

# Optionally include tool version checks, not used in Docker builds
TOOL_VERSION_CHECK ?= 1

##### Variables #####

# Project variables
PROJECT_NAME       := osresourcemgr
PROJECT_NICKNAME   := osr
BINARY_NAME        := osrm

# Code Versions
VERSION            := $(shell cat VERSION)
VERSION_MAJOR      := $(shell cut -c 1 VERSION)
DOCKER_IMG_NAME    := $(PROJECT_NAME)
DOCKER_VERSION     ?= $(shell git branch --show-current | sed 's/\//_/g')
GIT_COMMIT         ?= $(shell git rev-parse HEAD)
GIT_TAG_PREFIX     := os-resource/v

# Test variables
# Set TEST_TARGET to '<testname1>' or '<testname1\|testname2>' to run specific tests or use regex '<testname.*>' - example:
#  TEST_TARGET='TestAuth.*' make test
TEST_TARGET        ?= .
# Default TEST_PKG, overwrite to select particular test pakages
TEST_PKG           ?= $(shell go list ./... | grep -v "cmd" | grep -v "internal/testing" | tr '\n' ,)
TEST_COVER         := atomic ./...
TEST_ARGS          := -globalLogLevel=error
TEST_USE_DB        := true

# Directory to clean specific to this project
DIR_TO_CLEAN        :=

# Include shared makefile
include ../common.mk

generate: buf-gen ## Generate all the code

lint: generate license yamllint hadolint go-lint mdlint ## Run all lint tools

test: go-test ## Run all tests

#### Sub-targets ####

$(OUT_DIR)/$(BINARY_NAME): $(SRC) $(DEPS) ## Build if sources or dependencies changed
	$(GOCMD) build $(GOEXTRAFLAGS) -o $(OUT_DIR)/$(BINARY_NAME) cmd/$(BINARY_NAME).go

go-run: build ## Run the resource manager
	$(GOCMD) run $(GOEXTRAFLAGS) ./cmd/$(BINARY_NAME).go

#### Unsupported targets ###
buf-update buf-lint buf-gen dependency dependency-check go-dependency: ## Unsupported target
	echo 'make "$@" is unsupported'
