// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

var (
	ErrAclRootVerityUnsupportedBuild = NewImageCustomizerError("ACLRootVerity:UnsupportedBuild",
		"ACL root-verity customization requires the acl_root_verity Go build tag")
	ErrAclRootVerityNotImplemented = NewImageCustomizerError("ACLRootVerity:NotImplemented",
		"ACL root-verity conversion is not implemented")
)
