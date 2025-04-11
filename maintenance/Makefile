# Maintenance Manager Makefile

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

.DEFAULT_GOAL := help
.PHONY: test lint license generate help

all: generate build lint test

# Optionally include tool version checks, not used in Docker builds
TOOL_VERSION_CHECK ?= 1

##### Variables #####

# Required Go Dependencies
GOLINT              := true
GOJUNITREPORT       := true
GOCOBERTURA         := true
PROTOCGENDOC        := true
BUF                 := true
PROTOCGENGO         := true
PROTOCGENGOGRPC     := true
PROTOCGENVALIDATEGO := true

# Project variables
PROJECT_NAME        := maintmgr
PROJECT_NICKNAME    := m
BINARY_NAME         := $(PROJECT_NAME)

# Code versions, tags, and so on
VERSION             := $(shell cat VERSION)
VERSION_MAJOR       := $(shell cut -c 1 VERSION)
DOCKER_IMG_NAME     := $(PROJECT_NAME)
DOCKER_VERSION      ?= $(shell git branch --show-current | sed 's/\//_/g')
GIT_COMMIT          ?= $(shell git rev-parse HEAD)
GIT_TAG_PREFIX      := maintenance/v

# Test variables
# Set TEST_TARGET to '<testname1>' or '<testname1\|testname2>' to run specific tests or use regex '<testname.*>' - example:
#  TEST_TARGET='TestAuth.*' make test
TEST_TARGET         ?= .
# Default TEST_PKG, overwrite to select particular test pakages
TEST_PKG            ?= $(shell go list ./... | grep -v "pkg/api" | grep -v "cmd" | tr '\n' ,)
TEST_COVER          := atomic ./...
TEST_USE_DB         := true

# Directory to clean specific to this project
DIR_TO_CLEAN 		 := $(APIPKG_DIR)/maintmgr/v1/*.go

# Include shared makefile
include ../common.mk

generate: buf-gen ## Generate types, server and client

lint: generate license go-lint hadolint buf-lint mdlint ## Run all linters

test: go-test ## Run all tests

#### Sub-targets ####

$(OUT_DIR)/$(BINARY_NAME): $(SRC) $(DEPS) ## Build if sources or dependencies changed
	$(GOCMD) fmt ./...
	$(GOCMD) build $(GOEXTRAFLAGS) -o $(OUT_DIR)/$(BINARY_NAME) cmd/$(BINARY_NAME)/main.go

go-run: build ## go run
	$(GOCMD) run $(GOEXTRAFLAGS) cmd/maintmgr/main.go

buf-update: common-buf-update ## Update buf modules

buf-gen: common-buf-gen ## Compile protoc files into code

buf-lint: common-buf-lint ## Lint and format protobuf files
