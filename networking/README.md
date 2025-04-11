# Networking Manager

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Get Started](#get-started)
- [Usage](#usage)
- [Functional Test](#functional-test)
- [Contribute](#contribute)

## Overview

The Networking Manager constantly verifies the correctness of the networking configuration and IP
uniqueness on of the Edge Nodes within a site.

The Networking Manager handles Network Resources representing the network infrastructure of a site and its
relative addressing information. Most of the network resources are unique within a single site,
but not globally unique (e.g., multiple sites may have the same IP Subnet definitions).
This repository contains the Networking Manager implementation.

## Features

- Top-down Edge Node networking configuration and constant correctness monitoring, via reconciliation
- Exemplar reconciliation implementation, can be taken for other elements.

To learn more about internals and software architecture, see
[Edge Infrastructure Manager developer documentation][inframanager-dev-guide-url].

## Get Started

Instructions on how to install and set up Networking Manger on your development machine.

### Dependencies

Firstly, please verify that all dependencies have been installed.

```bash
# Return errors if any dependency is missing
make dependency-check
```

This code requires the following tools to be installed on your development machine:

- [Go\* programming language](https://go.dev) - check [$GOVERSION_REQ](../version.mk)
- [golangci-lint](https://github.com/golangci/golangci-lint) - check [$GOLINTVERSION_REQ](../version.mk)
- [go-junit-report](https://github.com/jstemmer/go-junit-report) - check [$GOJUNITREPORTVERSION_REQ](../version.mk)

You can install Go dependencies by running `make go-dependency`.

### Build the Binary

Build the project as follows:

```bash
# Build go binary
make build
```

The binary is installed in the [$OUT_DIR](../common.mk) folder.

## Usage

This guide shows how to deploy Networking Manger for local development or testing.
For production deployments use the [Edge Infrastructure Manager charts][inframanager-charts].

> Note: To run the Networking Manager, Inventory must to be running as the manager needs to register as an inventory client.
> Please refer to the
> [Inventory instructions](https://github.com/open-edge-platform/infra-core/tree/main/inventory#usage)
> and [Database in Inventory](https://github.com/open-edge-platform/infra-core/blob/main/inventory/docs/database.md)
> for more information about how to run inventory.

### Run Networking Manger

```bash
make run
```

See the [documentation][user-guide-url] if you want to learn more about using Edge Orchestrator.

## Functional Test

Run the make target `test` to mock agents to perform the relative behaviors for host resources.

```bash
make test
```

For any issues see the [Troubleshooting guide][troubleshooting-url].

## Contribute

To learn how to contribute to the project, see the [contributor's guide][contributors-guide-url]. The project will
accept contributions through Pull-Requests (PRs). PRs must be built successfully by the CI pipeline, pass linters
verifications and the unit tests.

There are several convenience make targets to support developer activities, you can use `help` to see a list of makefile
targets. The following is a list of makefile targets that support developer activities:

- `generate` to generate the database schema, Go code, and the Python binding from the protobuf definition of the APIs
- `lint` to run a list of linting targets
- `mdlint` to run linting of this file.
- `test` to run the unit test
- `go-tidy` to update the Go dependencies and regenerate the `go.sum` file
- `build` to build the project and generate executable files
- `docker-build` to build the Inventory Docker container

[user-guide-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/get_started_guide/index.html
[inframanager-dev-guide-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/infra_manager/index.html
[contributors-guide-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html
[troubleshooting-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/troubleshooting/index.html
[inframanager-charts]: https://github.com/open-edge-platform/infra-charts
