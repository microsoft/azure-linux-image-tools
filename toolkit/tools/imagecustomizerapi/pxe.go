// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"net/url"
	"strings"
)

var PxeIsoDownloadProtocols = []string{"ftp://", "http://", "https://", "nfs://", "tftp://"}

// Iso defines how the generated iso media should be configured.
type Pxe struct {
	KernelCommandLine KernelCommandLine   `yaml:"kernelCommandLine" json:"kernelCommandLine,omitempty"`
	AdditionalFiles   AdditionalFileList  `yaml:"additionalFiles" json:"additionalFiles,omitempty"`
	InitramfsType     InitramfsImageType  `yaml:"initramfsType" json:"initramfsType,omitempty"`
	BootstrapBaseUrl  string              `yaml:"bootstrapBaseUrl" json:"bootstrapBaseUrl,omitempty"`
	BootstrapFileUrl  string              `yaml:"bootstrapFileUrl" json:"bootstrapFileUrl,omitempty"`
	KdumpBootFiles    *KdumpBootFilesType `yaml:"kdumpBootFiles" json:"kdumpBootFiles,omitempty"`
}

func IsValidPxeUrl(urlString string) error {
	if urlString == "" {
		return nil
	}

	_, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("invalid URL value (%s):\n%w", urlString, err)
	}

	protocolFound := false
	for _, protocol := range PxeIsoDownloadProtocols {
		if strings.HasPrefix(urlString, protocol) {
			protocolFound = true
			break
		}
	}
	if !protocolFound {
		return fmt.Errorf("unsupported iso image URL protocol in (%s). One of (%v) is expected", urlString, PxeIsoDownloadProtocols)
	}

	return nil
}

func (p *Pxe) IsValid() error {
	err := p.KernelCommandLine.IsValid()
	if err != nil {
		return fmt.Errorf("invalid kernelCommandLine: %w", err)
	}

	err = p.AdditionalFiles.IsValid()
	if err != nil {
		return fmt.Errorf("invalid additionalFiles:\n%w", err)
	}

	err = p.InitramfsType.IsValid()
	if err != nil {
		return fmt.Errorf("invalid initramfs type:\n%w", err)
	}

	if p.InitramfsType == InitramfsImageTypeFullOS {
		if p.BootstrapBaseUrl != "" || p.BootstrapFileUrl != "" {
			return fmt.Errorf("cannot specify either 'bootstrapBaseUrl' or 'bootstrapFileUrl' when the initramfs type is set to '%s'", InitramfsImageTypeFullOS)
		}
	}

	if p.BootstrapBaseUrl != "" && p.BootstrapFileUrl != "" {
		return fmt.Errorf("cannot specify both 'bootstrapBaseUrl' and 'bootstrapFileUrl' at the same time")
	}
	err = IsValidPxeUrl(p.BootstrapBaseUrl)
	if err != nil {
		return fmt.Errorf("invalid 'bootstrapBaseUrl' field value (%s):\n%w", p.BootstrapBaseUrl, err)
	}
	err = IsValidPxeUrl(p.BootstrapFileUrl)
	if err != nil {
		return fmt.Errorf("invalid 'bootstrapFileUrl' field value (%s):\n%w", p.BootstrapFileUrl, err)
	}

	if p.KdumpBootFiles != nil {
		err = p.KdumpBootFiles.IsValid()
		if err != nil {
			return fmt.Errorf("invalid kdumpBootFiles:\n%w", err)
		}
	}

	return nil
}
