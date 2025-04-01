# Operating System (OS) Resource Manager

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Get Started](#get-started)
- [Usage](#usage)
- [Functional Test](#functional-test)
- [Contribute](#contribute)

## Overview

This repo holds the OS Resource manager codebase for OS Resource manager.
This service is responsible for creating a new OS Resource whenever a new OS version is released
to the Release Service. Additionally, it will optionally link the new OS Resource to Edge Nodes currently
using previous versions of the same OS, ensuring seamless updates and version management.

## Features

- Periodic Monitoring of Released Operations Systems and profiles
- Creation of OS resources
- Automatic and Manual assignment of OS resources to instances.

To learn more about internals and software architecture, see
[Edge OS Resource manager developer documentation][inframanager-dev-guide-url].

## Architecture

The OS Resource Manager monitors the Release Service, periodically quering the Release Service
to retrieve all immutable OS manifest files and mutable OS manifests, within a configurable interval.
Upon retrieving the manifests, OS Resource Manager will parse them to extract information that identifies
the corresponding OS Resource.
OS Resource Manager will have a cache of OS Resources, Tenant Resourcesd, Provider Resources and Instance Resource.
The cache for Tenant Resourcesd, Provider Resources and Instance Resource will be updated based on notifications
from the Inventory. OS Resource Manager will search the cache to verify the existence of OS Resources per all tenants.
If any OS Resource is missing, OS Resource Manager will create it and add to the Inventory.

For more information please check the [architecture and internals page](docs/architecture-internals.md).

## Get Started

Instructions on how to install and set up Networking Manger on your development machine.

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

Build the project as follows:

```bash
# Build go binary
make build
```

The binary is installed in the [$OUT_DIR](../common.mk) folder.

## Usage

This guide shows how to deploy OS Resource Manger for local development or testing.
For production deployments use the [Edge Infrastructure Manager charts][inframanager-charts].

> Note: To run host manager, Inventory need to be running as the host manager need to register as an inventory client.
> Please refer to the TODO
> [instruction of Inventory](https://github.com/open-edge-platform/infra-core/tree/main/inventory#usage)
> and [Database in Inventory](https://github.com/open-edge-platform/infra-core/blob/main/inventory/docs/database.md)
> for more information about how to run inventory.

### Run OS Resource Manger

```bash
make run
```

See the [documentation][user-guide-url] if you want to learn more about using Edge Orchestrator.

For any issues see the [Troubleshooting guide][troubleshooting-url].

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

[user-guide-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/get_started_guide/gsg_content.html
[inframanager-dev-guide-url]: (https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/get_started_guide/gsg_content.html)
[contributors-guide-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/index.html
[troubleshooting-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/troubleshooting/troubleshooting.html
[inframanager-charts]: https://github.com/open-edge-platform/infra-charts
