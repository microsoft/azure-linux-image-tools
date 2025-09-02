// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"go.opentelemetry.io/otel"
)

var (
	// Hostname-related errors
	ErrHostnameWrite = NewImageCustomizerError("Hostname:Write", "failed to write hostname file")
)

func UpdateHostname(ctx context.Context, hostname string, imageChroot safechroot.ChrootInterface) error {
	if hostname == "" {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_hostname")
	defer span.End()

	logger.Log.Infof("Setting hostname (%s)", hostname)

	hostnameFilePath := filepath.Join(imageChroot.RootDir(), "etc/hostname")
	err := file.Write(hostname, hostnameFilePath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrHostnameWrite, err)
	}

	return nil
}
