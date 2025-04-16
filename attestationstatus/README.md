# Edge Infrastructure Attestation Status Resource Manager

The Attestation Status Resource Manager updates the [Trusted
Compute](https://github.com/open-edge-platform/trusted-compute) attestation
status fields on Instance resources for an Edge Node.

It receives attestation  status updates from the Privileged Attestation Pod
component running on the Edge Node, determines which Instance to update (Using
the SMBIOS UUID of the device), then updates Inventory with the status data,
which can then be queried by the northbound REST API or UI and displayed to the
user.

The southbound gRPC API used is defined in the api directory, and is secured
using TLS with JWT for authentication, and rego rules

## Developing

To perform development tasks on the Attestation Status Manager, several tools
are required:

- [Go\* programming language](https://go.dev), and lint/test tools:
  - [golangci-lint](https://github.com/golangci/golangci-lint)
  - [go-junit-report](https://github.com/jstemmer/go-junit-report)
  - [gocover-cobertura](https://github.com/boumenot/gocover-cobertura)

- [buf](https://github.com/bufbuild/buf) and gRPC generation tools
  - [protoc-gen-doc](https://github.com/pseudomuto/protoc-gen-doc)
  - [protoc-gen-go](https://pkg.go.dev/google.golang.org/protobuf)
  - [protoc-gen-go-grpc](https://pkg.go.dev/google.golang.org/grpc)
  - [protoc-gen-validate](https://pkg.go.dev/github.com/envoyproxy/protoc-gen-validate)

- [python 3](https://www.python.org) for testing and generation

- [Docker](https://docs.docker.com) for test and build

The versions required are specified in `../version.mk`, and you can verify that
they are installed using `make dependency-check`.

If a Go dependency is missing, you can install it with `make go-dependency`.

### Developer Loop

All development tasks should start using `make` targets.  Get a list of these
with `make help`.

Typically you would want to make a change, then run these commands to lint,
test, and locally build the artifact:

- `make lint`
- `make test` - note: requires `docker` to run a local Postgres Database
- `make build`

### Local testing of a binary

After running `make build` you will have a locally compiled version in
`out/attestationstatusmgr`

When the binary is first run, it will attempt to connect to an [Inventory
service](https://github.com/open-edge-platform/infra-core/tree/main/inventory)
running on the same system. See the inventory README for how to run this
locally.

You can then run the binary with:

```bash
./out/attestationstatusmgr -rbacRules rego/authz.rego
```

which will offer a southbound gRPC API on `0.0.0.0:50007`

## License, Contribution, and Support

Please see the [README in the parent directory](../README.md).
