// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type CrashDump struct {
	KeepKdumpBootFiles bool `yaml:"keepKdumpBootFiles" json:"keepKdumpBootFiles,omitempty"`
}

func (u *CrashDump) IsValid() error {
	return nil
}
