// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"go.opentelemetry.io/otel"
)

var (
	// Kernel check-related errors
	ErrKernelListRead      = NewImageCustomizerError("Kernel:ListRead", "failed to read installed kernels list")
	ErrKernelModuleDirRead = NewImageCustomizerError("Kernel:ModuleDirRead", "failed to read installed kernel module directory")
	ErrNoInstalledKernel   = NewImageCustomizerError("Kernel:NoInstalled", "no installed kernel found")
)

// Check if the user accidentally uninstalled the kernel package without installing a substitute package.
func checkForInstalledKernel(ctx context.Context, imageChroot *safechroot.Chroot) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "check_for_installed_kernel")
	defer span.End()
	kernelModulesDir := filepath.Join(imageChroot.RootDir(), "/lib/modules")

	kernels, err := os.ReadDir(kernelModulesDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrKernelListRead, err)
	}

	for _, kernel := range kernels {
		// There is a bug in Azure Linux 2.0, where uninstalling the kernel package doesn't remove the directory
		// /lib/modules/<ver>. Instead the directory is just emptied. So, ensure the directory isn't empty.
		files, err := os.ReadDir(filepath.Join(kernelModulesDir, kernel.Name()))
		if err != nil {
			return fmt.Errorf("%w (kernel='%s'):\n%w", ErrKernelModuleDirRead, kernel.Name(), err)
		}

		if len(files) > 0 {
			// Found at least 1 kernel.
			return nil
		}
	}

	return ErrNoInstalledKernel
}
