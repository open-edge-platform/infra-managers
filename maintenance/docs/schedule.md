# Maintenance Schedules

There are two types of Maintenance Schedule resources:

- Single Schedule - These are one time events
- Repeated Schedule - These happen over and over, on the configured time.

Schedule resources can currently reference either a single Site or a Host.

The Maintenance Manager on the Edge Orchestrator collaborates with the Platform Update Agent on the Edge Node agent and
allows an update procedure of an EN with immutable Edge Microvisor Toolkit and an EN with mutable Ubuntu OS.
A distinction will be made between two types of updates both on Edge Orchestrator and Edge Node level.
The Edge Orchestrator will accept different set of information needed for update between the mutable and immutable EN.
The PUA will also be able to distinguish the type of update either by the type of EN it runs on, or by the type
of information it receives from MM.

Based on the above we can consider that there will be two update flows supported by MM/PUA.

Mutable OS Update flow: Day 2 update of mutable Ubuntu OS using apt package manager (as per past releases).
Immutable OS Update flow: Day 2 update of immutable Edge Microvisor Toolkit via A/B partition swap and
install of new OS image.

This repository contains the Maintenance Manager implementation and the southbound API exposed to the Edge Nodes.
