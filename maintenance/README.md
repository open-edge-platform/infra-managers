# Edge Infrastructure Maintenance Manager

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Get Started](#get-started)
- [Usage](#usage)
- [Functional Test](#functional-test)
- [Contribute](#contribute)

## Overview

The Maintenance Manager service is designed to help manage maintenance tasks for Edge Nodes.
It acts as a bridge, passing down the maintenance and update requests (Schedules) to the
managed Edge Nodes. This service is responsible for ensuring that Edge Nodes can perform the required maintenance
and update tasks.

Maintenance Manager handles `schedule` resources used to model time-based events, such as administrative downtime,
maintenance windows, or other events that may happen either a single time or repeated on a schedule.

For more information on the schedule and how this trasnaltes on the Edge Node please check
the [Schedule](docs/schedule.md).

## Features

- Top-down Edge Node maintenance scheduling, one off or recurring at specific times
- Single or per-group Edge Node updates
- Mutable OS Update: Day 2 update of the mutable Ubuntu OS using APT package manager (as per past releases).
- Immutable OS Update: Day 2 update of the immutable Edge Microvisor Toolkit via A/B partition swap and installation of a new OS image.

## Get Started

Instructions on how to install and set up the Maintenance Manger on your development machine.

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
- [gocover-cobertura](https://github.com/boumenot/gocover-cobertura) - check [$GOCOBERTURAVERSION_REQ](../version.mk)
- [protoc-gen-doc](https://github.com/pseudomuto/protoc-gen-doc) - check [$PROTOCGENDOCVERSION_REQ](../version.mk)
- [buf](https://github.com/bufbuild/buf) - check [$BUFVERSION_REQ](../version.mk)
- [protoc-gen-go](https://pkg.go.dev/google.golang.org/protobuf) - check [$PROTOCGENGOVERSION_REQ](../version.mk)
- [protoc-gen-go-grpc](https://pkg.go.dev/google.golang.org/grpc) - check [$PROTOCGENGOGRPCVERSION_REQ](../version.mk)
- [protoc-gen-validate](https://pkg.go.dev/github.com/envoyproxy/protoc-gen-validate) - check [PROTOCGENVALIDATEGOVERSION_REQ](../version.mk)

You can install Go dependencies by running `make go-dependency`.

### Build the Binary

Build the project as follows:

```bash
# Build go binary
make build
```

The binary is installed in the [$OUT_DIR](../common.mk) folder.

## Usage

This guide shows how to deploy Maintenance Manger for local development or testing.
For production deployments use the [Edge Infrastructure Manager charts][inframanager-charts].

> Note: To run the Maintenance Manager, Inventory must be running as the manager needs to register as an Inventory client.
> Please refer to [Inventory instructions](https://github.com/open-edge-platform/infra-core/tree/main/inventory#usage)
> and [Database in Inventory](https://github.com/open-edge-platform/infra-core/blob/main/inventory/docs/database.md)
> for more information about how to run Inventory.

### Run Maintenance Manger

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

### Building Protobuf

Make sure that buf style is enforced for protobuf file by running:

```bash
make buf-lint
```

> note: Please refer to [buf style guide](https://docs.buf.build/best-practices/style-guide) for best practices.

To generate Golang code from protobuf files, run:

```bash
make buf-gen
```

Buf can also lint and reformat protobuf files. If the buf-lint target fails, please fix any errors and reformat with:

```bash
buf format -w
```

For additional information:

- See the [docs](docs/api/maintmgr.md) for the Host Maintenance Manager APIs
- See[Edge Infrastructure Manager developer documentation][inframanager-dev-guide-url] for internals and
  software architecture.

[user-guide-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/get_started_guide/index.html
[inframanager-dev-guide-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/infra_manager/index.html
[contributors-guide-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html
[troubleshooting-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/troubleshooting/index.html
[inframanager-charts]: https://github.com/open-edge-platform/infra-charts
