# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

SHELL	:= bash -eu -o pipefail

SUBPROJECTS := host networking maintenance os-resource telemetry attestationstatus

.DEFAULT_GOAL := help
.PHONY: all build clean clean-all help lint test

# Repo root directory, where base makefiles are located
REPO_ROOT := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

#### Python venv Target ####
VENV_DIR := venv_managers

$(VENV_DIR): requirements.txt ## Create Python venv
	python3 -m venv $@ ;\
  set +u; . ./$@/bin/activate; set -u ;\
  python -m pip install --upgrade pip ;\
  python -m pip install -r requirements.txt

all: build lint test ## Runs build, lint, test stages for all subprojects

dependency-check: $(VENV_DIR)

lint: $(VENV_DIR) mdlint license ## lint common and all subprojects
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir lint; done

MD_FILES := $(shell find . -type f \( -name '*.md' \) -print )
mdlint: ## lint all markdown README.md files
	markdownlint --version
	markdownlint *.md

license: $(VENV_DIR) ## Check licensing with the reuse tool
	set +u; . ./$</bin/activate; set -u ;\
  reuse --version ;\
  reuse --root . lint

build: ## build in all subprojects
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir build; done

docker-build: ## build all docker containers
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir $@; done

docker-push: ## push all docker containers
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir $@; done

docker-list: ## list all docker containers
	@echo "images:"
	@for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir $@; done

test: ## test in all subprojects
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir test; done

clean: ## clean in all subprojects
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir clean; done

clean-all: ## clean-all in all subprojects, and delete virtualenv
	for dir in $(SUBPROJECTS); do $(MAKE) -C $$dir clean-all; done
	rm -rf $(VENV_DIR)

h-%: ## Runs host subproject's tasks, e.g. h-test
	$(MAKE) -C host $*

n-%: ## Runs networking subproject's tasks, e.g. n-test
	$(MAKE) -C networking $*

m-%: ## Runs maintenance subproject's tasks, e.g. m-test
	$(MAKE) -C maintenance $*

osr-%: ## Runs os-resource subproject's tasks, e.g. osr-test
	$(MAKE) -C os-resource $*

t-%: ## Runs telemetry subproject's tasks, e.g. t-test
	$(MAKE) -C telemetry $*

a-%: ## Runs attestationstatus subproject's tasks, e.g. a-test
	$(MAKE) -C attestationstatus $*

#### Help Target ####
help: ## print help for each target
	@echo infra-managers make targets
	@echo "Target               Makefile:Line    Description"
	@echo "-------------------- ---------------- -----------------------------------------"
	@grep -H -n '^[[:alnum:]%_-]*:.* ##' $(MAKEFILE_LIST) \
    | sort -t ":" -k 3 \
    | awk 'BEGIN  {FS=":"}; {sub(".* ## ", "", $$4)}; {printf "%-20s %-16s %s\n", $$3, $$1 ":" $$2, $$4};'
