# Edge Infrastructure Managers

## Overview

The repository includes different Managers, many of which communicate on the southbound with Edge Nodes.
The Managers are implemented as micro-services part of the Edge Infrastructure Manager of the Edge Manageability Framework.
For more information about Resource Manger please read the TODO [resource managers architecture][resource-managers-architecture-url].

## Get Started

The repository comprises the following components and services:

- [**Host Resource Manager**](host/): manage a host’s hardware information, provides the interfaces to other
  components to fetch such information. It also implements connection tracking and reconciliation.
- [**Maintenance Manager**](maintenance/): manages maintenance tasks for Edge Nodes' software updates.
- [**Networking Manager**](networking/): verifies network configuration and IP correctness and uniqueness for Edge
  Nodes within a site.
- [**OS Resource manager**](os-resource/):  manages OS Resources and plan Edge Node updates, based on new OS version
  releases .
- [**Telemetry Manager**](telemetry-manager/): manages Telemetry configuration on the different Edge Nodes.

Read more about Edge Orchestrator in the TODO [User Guide][user-guide-url].

## Develop

To develop one of the Managers please follow it's specific guide present in the README.md of its specific folder.

## Contribute

To learn how to contribute to the project, see the \[Contributor's
Guide\](<https://website-name.com>).

## Community and Support

To learn more about the project, its community, and governance, visit
the \[Edge Orchestrator Community\](<https://website-name.com>).

For support, start with \[Troubleshooting\](<https://website-name.com>) or
\[contact us\](<https://website-name.com>).

## License

Each component of the Edge Infrastructure core is licensed under
[Apache 2.0][apache-license].

Last Updated Date: April 7, 2025

[user-guide-url]: https://literate-adventure-7vjeyem.pages.github.io/edge_orchestrator/user_guide_main/content/user_guide/get_started_guide/gsg_content.html
[apache-license]: https://www.apache.org/licenses/LICENSE-2.0
