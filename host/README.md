# Edge Infrastructure Host Resource Manager

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Get Started](#get-started)
- [Usage](#usage)
- [Functional Test](#functional-test)
- [Contribute](#contribute)

## Overview

The purpose of the Host Manager is to manage a host’s hardware information and It also implements connection tracking
and reconciliation.
Host Resource Manager collects all of this data, such as CPU, Memory, Disk, GPU, Interfaces and such, with the help
of Hardware Discovery Agent (HDA) on the Edge Node.
The Host Resource Manager stores this data in inventory so that other components (like UI and Cluster Orchestration)
can retrieve and act on it.

Host Manager uses TLS with JWT (Json Web Tokens) technology to make the communication from the orchestrator to the edge
node secure.

## Features

- Discovery of all device's information: CPU, Memory, Disk, GPU, Interfaces, Peripherals and state.
- Connection tracking with reconciliation
- Scalable up to 10k of edge devices

## Get Started

Instructions on how to install and set up host Resource Manager on your development machine.

### Dependencies

Firstly, please verify that all dependencies have been installed.

```bash
# Return errors if any dependency is missing
make dependency-check
```

This code requires the following tools to be installed on your development machine:

- [Go\* programming language](https://go.dev) - check [$GOVERSION_REQ](Makefile)
- [golangci-lint](https://github.com/golangci/golangci-lint) - check [$GOLINTVERSION_REQ](Makefile)
- [go-junit-report](https://github.com/jstemmer/go-junit-report) - check [$GOJUNITREPORTVERSION_REQ](Makefile)
- [gocover-cobertura](github.com/boumenot/gocover-cobertura) - check [$GOCOBERTURAVERSION_REQ](Makefile)
- [protoc-gen-doc](https://github.com/pseudomuto/protoc-gen-doc) - check [$PROTOCGENDOCVERSION_REQ](Makefile)
- [buf](https://github.com/bufbuild/buf) - check [$BUFVERSION_REQ](Makefile)
- [protoc-gen-go](https://pkg.go.dev/google.golang.org/protobuf) - check [$PROTOCGENGOVERSION_REQ](Makefile)
- [protoc-gen-go-grpc](https://pkg.go.dev/google.golang.org/grpc) - check [$PROTOCGENGOGRPCVERSION_REQ](Makefile)
- [gnostic](https://pkg.go.dev/github.com/google/gnostic) - check [GNOSTICVERSION_REQ](Makefile)
- [protoc-gen-validate](https://pkg.go.dev/github.com/envoyproxy/protoc-gen-validate) - check [PROTOCGENVALIDATEGOVERSION_REQ](Makefile)
- [gnostic-grpc](https://pkg.go.dev/github.com/googleapis/gnostic-grpc) - check [GNOSTICGRPCVERSION_REQ](Makefile)
- [protoc-gen-grpc-gateway](https://pkg.go.dev/github.com/grpc-ecosystem/grpc-gateway/v2@v2.26.0/protoc-gen-grpc-gateway)
  - check [PROTOCGENGRPCGATEWAY_REQ](Makefile)

You can install Go dependencies by running `make go-dependency`.

### Build the Binary

Build the project as follows, generating the docker image of the Host Resource Manager.

```bash
# Build go binary
make build
```

The binary is installed in the [$OUT_DIR](../common.mk) folder.

## Usage

This guide shows how to deploy Host Resource Manger for local development or testing.
For production deployments use the [Edge Infrastructure Manager charts][inframanager-charts].

> Note: To run host manager, Inventory need to be running as the host manager need to register as an inventory client.
> Please refer to the TODO
> [instruction of Inventory](https://github.com/open-edge-platform/infra-core/tree/main/inventory#usage)
> and [Database in Inventory](https://github.com/open-edge-platform/infra-core/blob/main/inventory/docs/database.md)
> for more information about how to run inventory.

### Run Host Resource Manager

```bash
make go-run
```

See the [documentation][user-guide-url] if you want to learn more about using Edge Orchestrator.

For any issues the [Troubleshooting guide][troubleshooting-url].

## Functional Test

Run the make target `test` to mock agents to perform the relative behaviors for host resources.

```bash
make test
```

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

For additional information:

- See the [docs](docs/api/hostmgr.md) for the Host Resource Manager APIs
- See[Edge Infrastructure Manager developer documentation][inframanager-dev-guide-url] for internals and
  software architecture.

[user-guide-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/get_started_guide/gsg_content.html
[inframanager-dev-guide-url]: (https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/get_started_guide/gsg_content.html)
[contributors-guide-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/index.html
[troubleshooting-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/troubleshooting/troubleshooting.html
[inframanager-charts]: https://github.com/open-edge-platform/infra-charts
