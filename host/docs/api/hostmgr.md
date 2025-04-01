# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [hostmgr/proto/hostmgr_southbound.proto](#hostmgr_proto_hostmgr_southbound-proto)
    - [BiosInfo](#hostmgr_southbound_proto-BiosInfo)
    - [BmInfo](#hostmgr_southbound_proto-BmInfo)
    - [BmcInfo](#hostmgr_southbound_proto-BmcInfo)
    - [CPUTopology](#hostmgr_southbound_proto-CPUTopology)
    - [Config](#hostmgr_southbound_proto-Config)
    - [CoreGroup](#hostmgr_southbound_proto-CoreGroup)
    - [HWInfo](#hostmgr_southbound_proto-HWInfo)
    - [HostStatus](#hostmgr_southbound_proto-HostStatus)
    - [HostStatusResp](#hostmgr_southbound_proto-HostStatusResp)
    - [IPAddress](#hostmgr_southbound_proto-IPAddress)
    - [Interfaces](#hostmgr_southbound_proto-Interfaces)
    - [Metadata](#hostmgr_southbound_proto-Metadata)
    - [OsInfo](#hostmgr_southbound_proto-OsInfo)
    - [OsKernel](#hostmgr_southbound_proto-OsKernel)
    - [OsRelease](#hostmgr_southbound_proto-OsRelease)
    - [Socket](#hostmgr_southbound_proto-Socket)
    - [Storage](#hostmgr_southbound_proto-Storage)
    - [SystemCPU](#hostmgr_southbound_proto-SystemCPU)
    - [SystemDisk](#hostmgr_southbound_proto-SystemDisk)
    - [SystemGPU](#hostmgr_southbound_proto-SystemGPU)
    - [SystemInfo](#hostmgr_southbound_proto-SystemInfo)
    - [SystemMemory](#hostmgr_southbound_proto-SystemMemory)
    - [SystemNetwork](#hostmgr_southbound_proto-SystemNetwork)
    - [SystemPCI](#hostmgr_southbound_proto-SystemPCI)
    - [SystemUSB](#hostmgr_southbound_proto-SystemUSB)
    - [UpdateHostStatusByHostGuidRequest](#hostmgr_southbound_proto-UpdateHostStatusByHostGuidRequest)
    - [UpdateHostSystemInfoByGUIDRequest](#hostmgr_southbound_proto-UpdateHostSystemInfoByGUIDRequest)
    - [UpdateHostSystemInfoByGUIDResponse](#hostmgr_southbound_proto-UpdateHostSystemInfoByGUIDResponse)
    - [UpdateInstanceStateStatusByHostGUIDRequest](#hostmgr_southbound_proto-UpdateInstanceStateStatusByHostGUIDRequest)
    - [UpdateInstanceStateStatusByHostGUIDResponse](#hostmgr_southbound_proto-UpdateInstanceStateStatusByHostGUIDResponse)
  
    - [BmInfo.Bm_type](#hostmgr_southbound_proto-BmInfo-Bm_type)
    - [ConfigMode](#hostmgr_southbound_proto-ConfigMode)
    - [HostStatus.Host_status](#hostmgr_southbound_proto-HostStatus-Host_status)
    - [HostStatusResp.Host_action](#hostmgr_southbound_proto-HostStatusResp-Host_action)
    - [InstanceState](#hostmgr_southbound_proto-InstanceState)
    - [InstanceStatus](#hostmgr_southbound_proto-InstanceStatus)
  
    - [Hostmgr](#hostmgr_southbound_proto-Hostmgr)
  
- [Scalar Value Types](#scalar-value-types)



<a name="hostmgr_proto_hostmgr_southbound-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## hostmgr/proto/hostmgr_southbound.proto
SPDX-FileCopyrightText: (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0


<a name="hostmgr_southbound_proto-BiosInfo"></a>

### BiosInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  |  |
| release_date | [string](#string) |  |  |
| vendor | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-BmInfo"></a>

### BmInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| bm_type | [BmInfo.Bm_type](#hostmgr_southbound_proto-BmInfo-Bm_type) |  |  |
| bmc_info | [BmcInfo](#hostmgr_southbound_proto-BmcInfo) |  |  |






<a name="hostmgr_southbound_proto-BmcInfo"></a>

### BmcInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| bm_ip | [string](#string) |  |  |
| bm_username | [string](#string) |  |  |
| bm_password | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-CPUTopology"></a>

### CPUTopology



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| sockets | [Socket](#hostmgr_southbound_proto-Socket) | repeated | a list of CPU socket descriptions |






<a name="hostmgr_southbound_proto-Config"></a>

### Config



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-CoreGroup"></a>

### CoreGroup



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| core_type | [string](#string) |  | type of CPU cores (e.g., P-Core or E-Core) |
| core_list | [uint32](#uint32) | repeated | a list of CPU cores in the group |






<a name="hostmgr_southbound_proto-HWInfo"></a>

### HWInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| serial_num | [string](#string) |  |  |
| product_name | [string](#string) |  |  |
| cpu | [SystemCPU](#hostmgr_southbound_proto-SystemCPU) |  |  |
| gpu_deprecated | [SystemGPU](#hostmgr_southbound_proto-SystemGPU) |  | **Deprecated.**  |
| memory | [SystemMemory](#hostmgr_southbound_proto-SystemMemory) |  |  |
| storage | [Storage](#hostmgr_southbound_proto-Storage) |  |  |
| network | [SystemNetwork](#hostmgr_southbound_proto-SystemNetwork) | repeated |  |
| pci | [SystemPCI](#hostmgr_southbound_proto-SystemPCI) | repeated |  |
| usb | [SystemUSB](#hostmgr_southbound_proto-SystemUSB) | repeated |  |
| gpu | [SystemGPU](#hostmgr_southbound_proto-SystemGPU) | repeated |  |






<a name="hostmgr_southbound_proto-HostStatus"></a>

### HostStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_status | [HostStatus.Host_status](#hostmgr_southbound_proto-HostStatus-Host_status) |  |  |
| details | [string](#string) |  |  |
| human_readable_status | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-HostStatusResp"></a>

### HostStatusResp



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_action | [HostStatusResp.Host_action](#hostmgr_southbound_proto-HostStatusResp-Host_action) |  |  |
| details | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-IPAddress"></a>

### IPAddress



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ip_address | [string](#string) |  | Just an IP Address of the interface (e.g., 192.168.1.12) |
| network_prefix_bits | [int32](#int32) |  |  |
| config_mode | [ConfigMode](#hostmgr_southbound_proto-ConfigMode) |  | this is derived from the FLAG and lifetime associated with the IPAddress |






<a name="hostmgr_southbound_proto-Interfaces"></a>

### Interfaces



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| class | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-Metadata"></a>

### Metadata



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-OsInfo"></a>

### OsInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| kernel | [OsKernel](#hostmgr_southbound_proto-OsKernel) |  |  |
| release | [OsRelease](#hostmgr_southbound_proto-OsRelease) |  |  |






<a name="hostmgr_southbound_proto-OsKernel"></a>

### OsKernel



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  |  |
| config | [Config](#hostmgr_southbound_proto-Config) | repeated |  |






<a name="hostmgr_southbound_proto-OsRelease"></a>

### OsRelease



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| version | [string](#string) |  |  |
| metadata | [Metadata](#hostmgr_southbound_proto-Metadata) | repeated |  |






<a name="hostmgr_southbound_proto-Socket"></a>

### Socket



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| socket_id | [uint32](#uint32) |  |  |
| core_groups | [CoreGroup](#hostmgr_southbound_proto-CoreGroup) | repeated | a list of CPU core groups, categorized by CPU core type |






<a name="hostmgr_southbound_proto-Storage"></a>

### Storage



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| disk | [SystemDisk](#hostmgr_southbound_proto-SystemDisk) | repeated |  |
| features | [string](#string) | repeated |  |






<a name="hostmgr_southbound_proto-SystemCPU"></a>

### SystemCPU



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| arch | [string](#string) |  |  |
| vendor | [string](#string) |  |  |
| model | [string](#string) |  |  |
| sockets | [uint32](#uint32) |  |  |
| cores | [uint32](#uint32) |  |  |
| threads | [uint32](#uint32) |  |  |
| features | [string](#string) | repeated |  |
| cpu_topology | [CPUTopology](#hostmgr_southbound_proto-CPUTopology) |  |  |






<a name="hostmgr_southbound_proto-SystemDisk"></a>

### SystemDisk



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| serial_number | [string](#string) |  |  |
| name | [string](#string) |  |  |
| vendor | [string](#string) |  |  |
| model | [string](#string) |  |  |
| size | [uint64](#uint64) |  |  |
| wwid | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-SystemGPU"></a>

### SystemGPU



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pci_id | [string](#string) |  |  |
| product | [string](#string) |  |  |
| vendor | [string](#string) |  |  |
| name | [string](#string) |  |  |
| description | [string](#string) |  | human-readable description of GPU |
| features | [string](#string) | repeated |  |






<a name="hostmgr_southbound_proto-SystemInfo"></a>

### SystemInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hw_info | [HWInfo](#hostmgr_southbound_proto-HWInfo) |  |  |
| os_info | [OsInfo](#hostmgr_southbound_proto-OsInfo) |  |  |
| bm_ctl_info | [BmInfo](#hostmgr_southbound_proto-BmInfo) |  |  |
| bios_info | [BiosInfo](#hostmgr_southbound_proto-BiosInfo) |  |  |






<a name="hostmgr_southbound_proto-SystemMemory"></a>

### SystemMemory



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| size | [uint64](#uint64) |  |  |






<a name="hostmgr_southbound_proto-SystemNetwork"></a>

### SystemNetwork



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| pci_id | [string](#string) |  |  |
| mac | [string](#string) |  |  |
| link_state | [bool](#bool) |  |  |
| current_speed | [uint64](#uint64) |  |  |
| current_duplex | [string](#string) |  |  |
| supported_link_mode | [string](#string) | repeated |  |
| advertising_link_mode | [string](#string) | repeated |  |
| features | [string](#string) | repeated |  |
| sriovenabled | [bool](#bool) |  |  |
| sriovnumvfs | [uint32](#uint32) |  |  |
| sriov_vfs_total | [uint32](#uint32) |  |  |
| peer_name | [string](#string) |  |  |
| peer_description | [string](#string) |  |  |
| peer_mac | [string](#string) |  |  |
| peer_mgmt_ip | [string](#string) |  |  |
| peer_port | [string](#string) |  |  |
| ip_addresses | [IPAddress](#hostmgr_southbound_proto-IPAddress) | repeated | NIC can report multiple IP addresses for each NIC |
| mtu | [uint32](#uint32) |  | units are bytes |
| bmc_net | [bool](#bool) |  | whether or not this is a bmc NIC |






<a name="hostmgr_southbound_proto-SystemPCI"></a>

### SystemPCI



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dev_class | [string](#string) |  |  |






<a name="hostmgr_southbound_proto-SystemUSB"></a>

### SystemUSB



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| class | [string](#string) |  |  |
| idvendor | [string](#string) |  |  |
| idproduct | [string](#string) |  |  |
| bus | [uint32](#uint32) |  |  |
| addr | [uint32](#uint32) |  |  |
| description | [string](#string) |  |  |
| serial | [string](#string) |  |  |
| interfaces | [Interfaces](#hostmgr_southbound_proto-Interfaces) | repeated |  |






<a name="hostmgr_southbound_proto-UpdateHostStatusByHostGuidRequest"></a>

### UpdateHostStatusByHostGuidRequest
UpdateHostStatusByHostGuidParameters holds parameters to UpdateHostStatusByHostGuid


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_guid | [string](#string) |  |  |
| host_status | [HostStatus](#hostmgr_southbound_proto-HostStatus) |  |  |






<a name="hostmgr_southbound_proto-UpdateHostSystemInfoByGUIDRequest"></a>

### UpdateHostSystemInfoByGUIDRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_guid | [string](#string) |  |  |
| system_info | [SystemInfo](#hostmgr_southbound_proto-SystemInfo) |  |  |






<a name="hostmgr_southbound_proto-UpdateHostSystemInfoByGUIDResponse"></a>

### UpdateHostSystemInfoByGUIDResponse







<a name="hostmgr_southbound_proto-UpdateInstanceStateStatusByHostGUIDRequest"></a>

### UpdateInstanceStateStatusByHostGUIDRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_guid | [string](#string) |  | Host GUID |
| instance_status | [InstanceStatus](#hostmgr_southbound_proto-InstanceStatus) |  | Instance&#39;s Status |
| instance_state | [InstanceState](#hostmgr_southbound_proto-InstanceState) |  | Instance&#39;s last State as seen by the PS/ENA |
| provider_status_detail | [string](#string) |  | Details of the current status of the Instance |






<a name="hostmgr_southbound_proto-UpdateInstanceStateStatusByHostGUIDResponse"></a>

### UpdateInstanceStateStatusByHostGUIDResponse






 


<a name="hostmgr_southbound_proto-BmInfo-Bm_type"></a>

### BmInfo.Bm_type
buf:lint:ignore ENUM_VALUE_PREFIX
buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
buf:lint:ignore ENUM_PASCAL_CASE

| Name | Number | Description |
| ---- | ------ | ----------- |
| IPMI | 0 |  |
| REDFISH | 1 |  |
| PDU | 2 |  |
| VPRO | 3 |  |
| FDO | 4 |  |
| NONE | 5 |  |



<a name="hostmgr_southbound_proto-ConfigMode"></a>

### ConfigMode


| Name | Number | Description |
| ---- | ------ | ----------- |
| CONFIG_MODE_UNSPECIFIED | 0 |  |
| CONFIG_MODE_STATIC | 1 |  |
| CONFIG_MODE_DYNAMIC | 2 |  |



<a name="hostmgr_southbound_proto-HostStatus-Host_status"></a>

### HostStatus.Host_status
buf:lint:ignore ENUM_VALUE_PREFIX
buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
buf:lint:ignore ENUM_PASCAL_CASE

| Name | Number | Description |
| ---- | ------ | ----------- |
| UNSPECIFIED | 0 |  |
| REGISTERING | 1 |  |
| BOOTING | 2 |  |
| BOOTFAILED | 3 |  |
| PROVISIONING | 4 |  |
| PROVISIONED | 5 |  |
| PROVISIONFAILED | 6 |  |
| RUNNING | 7 |  |
| UPDATING | 8 |  |
| UPDATEFAILED | 9 |  |
| ERROR | 10 |  |



<a name="hostmgr_southbound_proto-HostStatusResp-Host_action"></a>

### HostStatusResp.Host_action
buf:lint:ignore ENUM_VALUE_PREFIX
buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
buf:lint:ignore ENUM_PASCAL_CASE

| Name | Number | Description |
| ---- | ------ | ----------- |
| NONE | 0 |  |
| SHUTDOWN | 1 |  |
| RESTART | 2 |  |
| UPDATING | 3 |  |
| RUNNING | 4 |  |



<a name="hostmgr_southbound_proto-InstanceState"></a>

### InstanceState


| Name | Number | Description |
| ---- | ------ | ----------- |
| INSTANCE_STATE_UNSPECIFIED | 0 | unconfigured |
| INSTANCE_STATE_INSTALLED | 2 | OS is installed, but hasn&#39;t been started |
| INSTANCE_STATE_RUNNING | 3 | OS is Running |
| INSTANCE_STATE_STOPPED | 4 | OS is Stopped |
| INSTANCE_STATE_DELETED | 5 | OS should be Deleted |



<a name="hostmgr_southbound_proto-InstanceStatus"></a>

### InstanceStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| INSTANCE_STATUS_UNSPECIFIED | 0 |  |
| INSTANCE_STATUS_BOOTING | 1 |  |
| INSTANCE_STATUS_BOOT_FAILED | 2 |  |
| INSTANCE_STATUS_PROVISIONING | 3 |  |
| INSTANCE_STATUS_PROVISIONED | 4 |  |
| INSTANCE_STATUS_PROVISION_FAILED | 5 |  |
| INSTANCE_STATUS_RUNNING | 6 |  |
| INSTANCE_STATUS_ERROR | 7 |  |
| INSTANCE_STATUS_UPDATING | 9 |  |
| INSTANCE_STATUS_UPDATE_FAILED | 10 |  |


 

 


<a name="hostmgr_southbound_proto-Hostmgr"></a>

### Hostmgr
buf:lint:ignore SERVICE_SUFFIX

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| UpdateHostStatusByHostGuid | [UpdateHostStatusByHostGuidRequest](#hostmgr_southbound_proto-UpdateHostStatusByHostGuidRequest) | [HostStatusResp](#hostmgr_southbound_proto-HostStatusResp) | buf:lint:ignore RPC_RESPONSE_STANDARD_NAME |
| UpdateInstanceStateStatusByHostGUID | [UpdateInstanceStateStatusByHostGUIDRequest](#hostmgr_southbound_proto-UpdateInstanceStateStatusByHostGUIDRequest) | [UpdateInstanceStateStatusByHostGUIDResponse](#hostmgr_southbound_proto-UpdateInstanceStateStatusByHostGUIDResponse) | This call is dedicated to updating of an Instance&#39;s Current State AND Instance&#39;s Status. |
| UpdateHostSystemInfoByGUID | [UpdateHostSystemInfoByGUIDRequest](#hostmgr_southbound_proto-UpdateHostSystemInfoByGUIDRequest) | [UpdateHostSystemInfoByGUIDResponse](#hostmgr_southbound_proto-UpdateHostSystemInfoByGUIDResponse) | This call will update the Host System info |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

