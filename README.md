# Edge Infrastructure Managers

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/open-edge-platform/infra-managers/badge)](https://scorecard.dev/viewer/?uri=github.com/open-edge-platform/infra-managers)

## Overview

The repository includes different Managers, many of which communicate on the southbound with Edge Nodes.
The Managers are implemented as micro-services part of the Edge Infrastructure Manager of the Edge Manageability Framework.
For more information about Resource Manger please read the [resource managers architecture][resource-managers-architecture-url].

## Get Started

The repository comprises the following components and services:

- [**Host Resource Manager**](host/): manages a host’s hardware information, provides the interfaces to other
  components to fetch such information. It also implements connection tracking and reconciliation.
- [**Maintenance Manager**](maintenance/): manages maintenance tasks for Edge Nodes' software updates.
- [**Networking Manager**](networking/): verifies network configuration and IP correctness and uniqueness for Edge
  Nodes within a site.
- [**OS Resource manager**](os-resource/):  manages OS Resources and plan Edge Node updates, based on new OS version
  releases .
- [**Telemetry Manager**](telemetry/): manages Telemetry configuration on the different Edge Nodes.

Read more about Edge Orchestrator in the [User Guide](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/index.html).

## Develop

To develop one of the Managers, please follow its guide in README.md located in its respective folder.

## Contribute

To learn how to contribute to the project, see the [Contributor's
Guide](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html).

## Community and Support

To learn more about the project, its community, and governance, visit
the [Edge Orchestrator Community](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/index.html).

For support, start with [Troubleshooting](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/troubleshooting/index.html)

## License

Each component of the Edge Infrastructure managers is licensed under [Apache 2.0][apache-license].

Last Updated Date: April 7, 2025

[apache-license]: https://www.apache.org/licenses/LICENSE-2.0
[resource-managers-architecture-url]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/infra_manager/arch/orchestrator/architecture.html
