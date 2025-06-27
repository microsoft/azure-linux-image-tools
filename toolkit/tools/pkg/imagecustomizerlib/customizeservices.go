// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/systemd"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func EnableOrDisableServices(ctx context.Context, services imagecustomizerapi.Services, imageChroot safechroot.ChrootInterface) error {
	var err error

	if len(services.Enable) == 0 && len(services.Disable) == 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "configure_services")
	span.SetAttributes(
		attribute.Int("services_enable_count", len(services.Enable)),
		attribute.Int("services_disable_count", len(services.Disable)),
	)
	defer span.End()

	// Handle enabling services
	for _, service := range services.Enable {
		logger.Log.Infof("Enabling service (%s)", service)

		err = imageChroot.UnsafeRun(func() error {
			return shell.ExecuteLiveWithErr(1, "systemctl", "enable", service)
		})
		if err != nil {
			return fmt.Errorf("failed to enable service (%s):\n%w", service, err)
		}
	}

	// Handle disabling services
	for _, service := range services.Disable {
		logger.Log.Infof("Disabling service (%s)", service)

		// `systemctl disable` does not seem to fail when the service does not exist when running under chroot.
		// So, use `systemctl is-enabled` to check if the service exists.
		_, err := systemd.IsServiceEnabled(service, imageChroot)
		if err != nil {
			return fmt.Errorf("failed to disable service (%s):\n%w", service, err)
		}

		err = imageChroot.UnsafeRun(func() error {
			return shell.ExecuteLiveWithErr(1, "systemctl", "disable", service)
		})
		if err != nil {
			return fmt.Errorf("failed to disable service (%s):\n%w", service, err)
		}
	}

	return nil
}
