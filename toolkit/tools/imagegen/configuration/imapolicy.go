// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// ImaPolicy sets the ima_policy kernel command line option
type ImaPolicy string

const (
	// ImaPolicyTcb selects the tcb IMA policy
	ImaPolicyTcb ImaPolicy = "tcb"
	// ImaPolicyAppraiseTcb selects the appraise_tcb IMA policy
	ImaPolicyAppraiseTcb ImaPolicy = "appraise_tcb"
	// ImaPolicySecureBoot selects the secure_boot IMA policy
	ImaPolicySecureBoot ImaPolicy = "secure_boot"
	// ImaPolicyNone selects no IMA policy
	ImaPolicyNone ImaPolicy = ""
)
