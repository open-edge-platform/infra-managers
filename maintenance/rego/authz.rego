# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

package authz

import rego.v1

# This query checks if caller has write access to the resource
hasWriteAccess if {
    some role in input["realm_access/roles"] # iteration
    # We expect:
    # - with MT: [PROJECT_UUID]_en-agent-rw
    regex.match("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}_en-agent-rw$", role)
}

# This query checks if caller has read access to the resource
hasReadAccess if {
    some role in input["realm_access/roles"] # iteration
    # We expect:
    # - with MT: [PROJECT_UUID]_en-agent-rw
    regex.match("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}_en-agent-rw$", role)
}
