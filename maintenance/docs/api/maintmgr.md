# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [maintmgr/v1/maintmgr.proto](#maintmgr_v1_maintmgr-proto)
    - [OSProfileUpdateSource](#maintmgr-v1-OSProfileUpdateSource)
    - [PlatformUpdateStatusRequest](#maintmgr-v1-PlatformUpdateStatusRequest)
    - [PlatformUpdateStatusResponse](#maintmgr-v1-PlatformUpdateStatusResponse)
    - [RepeatedSchedule](#maintmgr-v1-RepeatedSchedule)
    - [SingleSchedule](#maintmgr-v1-SingleSchedule)
    - [UpdateSchedule](#maintmgr-v1-UpdateSchedule)
    - [UpdateSource](#maintmgr-v1-UpdateSource)
    - [UpdateStatus](#maintmgr-v1-UpdateStatus)
  
    - [PlatformUpdateStatusResponse.OSType](#maintmgr-v1-PlatformUpdateStatusResponse-OSType)
    - [UpdateStatus.StatusType](#maintmgr-v1-UpdateStatus-StatusType)
  
    - [MaintmgrService](#maintmgr-v1-MaintmgrService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="maintmgr_v1_maintmgr-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## maintmgr/v1/maintmgr.proto



<a name="maintmgr-v1-OSProfileUpdateSource"></a>

### OSProfileUpdateSource



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| os_image_url | [string](#string) |  | URL that PUA will use to download the immutable OS image file |
| os_image_id | [string](#string) |  | Identifier (e.g., version) of the image to be used by PUA to verify if it can use the OS image for EN update |
| os_image_sha | [string](#string) |  | Version of the image to be used by PUA to verify if it can use the OS image for EN update |
| profile_name | [string](#string) |  | Describes the OS profile name used by EN |
| profile_version | [string](#string) |  | Describes the OS profile version used by EN |






<a name="maintmgr-v1-PlatformUpdateStatusRequest"></a>

### PlatformUpdateStatusRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_guid | [string](#string) |  |  |
| update_status | [UpdateStatus](#maintmgr-v1-UpdateStatus) |  |  |






<a name="maintmgr-v1-PlatformUpdateStatusResponse"></a>

### PlatformUpdateStatusResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| update_source | [UpdateSource](#maintmgr-v1-UpdateSource) |  |  |
| update_schedule | [UpdateSchedule](#maintmgr-v1-UpdateSchedule) |  |  |
| installed_packages | [string](#string) |  | Freeform text, OS-dependent. A list of package names, one per line (newline separated). Should not contain version info. |
| os_type | [PlatformUpdateStatusResponse.OSType](#maintmgr-v1-PlatformUpdateStatusResponse-OSType) |  |  |
| os_profile_update_source | [OSProfileUpdateSource](#maintmgr-v1-OSProfileUpdateSource) |  |  |






<a name="maintmgr-v1-RepeatedSchedule"></a>

### RepeatedSchedule



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| duration_seconds | [uint32](#uint32) |  | between 1 second and 86400 seconds (24 hours worth of seconds) |
| cron_minutes | [string](#string) |  | cron style minutes (0-59) |
| cron_hours | [string](#string) |  | cron style hours (0-23) |
| cron_day_month | [string](#string) |  | cron style day of month (0-31) |
| cron_month | [string](#string) |  | cron style month (1-12) |
| cron_day_week | [string](#string) |  | cron style day of week (0-6) |






<a name="maintmgr-v1-SingleSchedule"></a>

### SingleSchedule



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| start_seconds | [uint64](#uint64) |  | start of one-time schedule (required) |
| end_seconds | [uint64](#uint64) |  | end of one-time schedule (optional) |






<a name="maintmgr-v1-UpdateSchedule"></a>

### UpdateSchedule



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| single_schedule | [SingleSchedule](#maintmgr-v1-SingleSchedule) |  |  |
| repeated_schedule | [RepeatedSchedule](#maintmgr-v1-RepeatedSchedule) |  | **Deprecated.**  |
| repeated_schedules | [RepeatedSchedule](#maintmgr-v1-RepeatedSchedule) | repeated | provide a list of repeated schedules to PUA. |






<a name="maintmgr-v1-UpdateSource"></a>

### UpdateSource



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| kernel_command | [string](#string) |  | Kernel command line |
| os_repo_url | [string](#string) |  | **Deprecated.** &#39;DEB822 Source Format&#39; url to the public repository, deprecated in 24.11 |
| custom_repos | [string](#string) | repeated | &#39;DEB822 Source Format&#39; entries for Debian style OSs |






<a name="maintmgr-v1-UpdateStatus"></a>

### UpdateStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status_type | [UpdateStatus.StatusType](#maintmgr-v1-UpdateStatus-StatusType) |  |  |
| status_detail | [string](#string) |  |  |
| profile_name | [string](#string) |  |  |
| profile_version | [string](#string) |  |  |
| os_image_id | [string](#string) |  |  |





 


<a name="maintmgr-v1-PlatformUpdateStatusResponse-OSType"></a>

### PlatformUpdateStatusResponse.OSType


| Name | Number | Description |
| ---- | ------ | ----------- |
| OS_TYPE_UNSPECIFIED | 0 |  |
| OS_TYPE_MUTABLE | 1 |  |
| OS_TYPE_IMMUTABLE | 2 |  |



<a name="maintmgr-v1-UpdateStatus-StatusType"></a>

### UpdateStatus.StatusType


| Name | Number | Description |
| ---- | ------ | ----------- |
| STATUS_TYPE_UNSPECIFIED | 0 | Default value, status not specified |
| STATUS_TYPE_UP_TO_DATE | 1 | Status when EN is not performing any update related actions. |
| STATUS_TYPE_STARTED | 2 | Status when the update process of EN has started |
| STATUS_TYPE_UPDATED | 3 | Status when the EN update is completed succesfully |
| STATUS_TYPE_FAILED | 4 | Status when the EN update fails; a detailed log is also sent |
| STATUS_TYPE_DOWNLOADING | 5 | Status when the EN is downloading update artifacts |
| STATUS_TYPE_DOWNLOADED | 6 | Status when the EN completes downloading update artifacts |


 

 


<a name="maintmgr-v1-MaintmgrService"></a>

### MaintmgrService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| PlatformUpdateStatus | [PlatformUpdateStatusRequest](#maintmgr-v1-PlatformUpdateStatusRequest) | [PlatformUpdateStatusResponse](#maintmgr-v1-PlatformUpdateStatusResponse) |  |

 



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

