# Telemetry Profile

Infrastructure management allows the Admin/User to configure what observability data is being collected
from the Edge Node. To achieve the same the user shall be given an option to
assign predefined "Telemetry profile"('s) to a Region or Site which then gets applied to all the
Edge Nodes associated with them.

To perform this configuration on multiple items metadata-hierachy is used
[Diagram of Hierarchy to Telemetry Mapping](./docs/telemetry-hierarchy.drawio.svg)

This document provides the list of pre-defined telemetry profiles (metrics and logs).

## Telemetry Groups

Pre-defined groups of telemetry data containing:

- Name of the Telemetry Group
- List of Metrics or Logs collected, given in the "Metrics collected" or "Logs collected" column below.

These are created during the installation process of Infrastructure Management software and are not condfigurable.

### Telemetry Profiles

These are used to assign a Telemetry Group to the heirarchy.

They contain:

- A reference to a Telemetry Group
- A reference to the Heirarchy object
  - Region or Site
  - Instance

- For Metrics, an Interval time value
- For Logs and Traces, a Severity value

## Metrics Profiles

| Profile name | Description                                                                                                                                                                                                                                    | Metrics collected <br> (Telegraf Plugin)| Host or Cluser Instance |
|:-------------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------|------|
| HW Usage | Generic hardware resource usage                                                                                                                                                                                                                | cpu <br> mem <br> disk | Host |
| CPU Performance and Health | Insight into performance and health of IA processor's internal components, <br/> including core and uncore units                                                                                                                               | intel_pmu | Host |
|Disk Performance  and Health | Indicators of drive (HDD, SSD) reliability. <br> Disk traffic.                                                                                                                                                                                 | smart <br> diskio | Host |
| GPU usage | Usage information of an Intel GPU                                                                                                                                                                                                              | intel_gpu_top (exec) | Host|
| Network Usage | Network interface and protocol usage <br> TCP connections state and UDP socket counts <br> Ethernet device stats                                                                                                                               | net <br> netstat <br> ethtool| Host |
| Power usage and Temperature | Enable monitoring platform Power, TDP and per-CPU metrics like temperature, <br/> power and utilization <br> Enable system temperature monitoring                                                                                              | intel_powerstat <br> temp | Host |
| Reliability, Availability Serviceability | Gather metrics provided by [rasdaemon](https://github.com/mchehab/rasdaemon)                                                                                                                                                                   | ras | Host |
| K8s - stats | Metrics derived from the state of Kubernetes resources (e.g. pods, deployments, ingress etc.) <br/> <br> Metrics about the running pods and containers                                                                                         | kube_inventory <br> kubernetes | Cluster |
| Redfish | Enables collection of metrics and status information about CPU temperature, fanspeed, Powersupply, <br/> voltage, hostname and Location details (datacenter, placement, rack and room) of hardware servers for which DMTF's Redfish is enabled | redfish | Host |
| Opentelemetry | Enable receiving traces, metrics and logs from OpenTelemetry clients and agents via gRPC                                                                                                                                                       | opentelemetry | Cluster |

### Interval options

Lists the metrics collection interval allowed on the Edge Node.

- 30 seconds
- 1 minute
- 5 minute
- 10 minute
- 30 minute
- 60 minute

## Logging Profiles

| Profile name | Description                                                                  | Logs collected | Host or Cluser Instance |
|:-------------|:-----------------------------------------------------------------------------|-------------------|-----|
| Fleet agents | Filtered Systemd Logs from infrastrucutre manager Bare Metal Agents          | lpke <br> HW agent <br> Cluster agent <br> Node agent <br> Vault agent <br> Platform Update agent <br> INBC | Host |
| Security | Filtered output of Apparmor status cmd (aa-status)                           | BMA agent status <br> vault status <br> fluentbit status | Host |
| Procs & Users | Filtered output of <br>ps, <br>cat /etc/passwd and <br>cat /var/log/auth.log | Process list <br> Users List <br> Authentication log | Host |
| Firewall | Firewall logs on Edge Node <br> output of ufw status cmd <>                  | Firewall status (ufw) <br> Firewall log (ufw.log) <br> Firewall sys log (syslog) <br>Firewall Kernel log (kern.log) | Host |
| SystemD | OS systemd logs                                                              | systemd | Host |
| Kernel | OS kernel logs                                                               | kmsg | Host |
| Container | Container logs <br> /var/log/containers/*.log                                | Container | Host |
| RKE | Rancher logs in K8s Edge Node                                                | RKE Server <br> RKE Agent <br> kubelet | Cluster |
| Opentelemetry |                                                                              | opentelemetry | Cluster |

### Logging level options

Lists the logging levels allowed on the Edge Node.

- CRITICAL
- ERROR
- WARN
- INFORMATION
- DEBUG

## Rendering Rules

The Telemetry Manager is responsible for performing this combining/rendering of
multiple Telemetry Profiles into a Combined Telemetry Profile that the Agent
can consume.

The following rules apply when combining fields:

- For group fields: the union of all the groups is used.

- For interval and latency fields: the lowest number is used.

- For level fields: the most verbose level is used.
