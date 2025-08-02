// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/unix"
)

var (
	// Script execution errors
	ErrScriptExecution        = NewImageCustomizerError("Scripts:Execution", "script execution failed")
	ErrScriptPathResolution   = NewImageCustomizerError("Scripts:PathResolution", "failed to get relative path for temp script file")
	ErrScriptTempFileRemoval  = NewImageCustomizerError("Scripts:TempFileRemoval", "failed to remove temp script file")
	ErrScriptTempFileCreation = NewImageCustomizerError("Scripts:TempFileCreation", "failed to create temp file for script")
	ErrScriptTempFileWrite    = NewImageCustomizerError("Scripts:TempFileWrite", "failed to write temp file for script")
	ErrScriptTempFileClose    = NewImageCustomizerError("Scripts:TempFileClose", "failed to close temp file for script")
)

const (
	configDirMountPathInChroot = "/_imageconfigs"
)

func runUserScripts(ctx context.Context, baseConfigPath string, scripts []imagecustomizerapi.Script, listName string,
	imageChroot *safechroot.Chroot,
) error {
	if len(scripts) <= 0 {
		return nil
	}

	logger.Log.Infof("Running %s scripts", listName)

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "run_"+listName+"_scripts")
	span.SetAttributes(
		attribute.Int("scripts_count", len(scripts)),
	)
	defer span.End()

	configDirMountPath := filepath.Join(imageChroot.RootDir(), configDirMountPathInChroot)

	// Bind mount the config directory so that the scripts can access any required resources.
	mount, err := safemount.NewMount(baseConfigPath, configDirMountPath, "", unix.MS_BIND|unix.MS_RDONLY, "", true)
	if err != nil {
		return err
	}
	defer mount.Close()

	// Runs scripts.
	for i, script := range scripts {
		err := runUserScript(i, script, listName, imageChroot)
		if err != nil {
			return err
		}
	}

	err = mount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func runUserScript(scriptIndex int, script imagecustomizerapi.Script, listName string,
	imageChroot *safechroot.Chroot,
) error {
	var err error

	scriptLogName := createScriptLogName(scriptIndex, script, listName)

	logger.Log.Infof("Running script (%s)", scriptLogName)

	// Collect the process name and args.
	scriptPath := ""
	tempScriptFullPath := ""
	if script.Path != "" {
		scriptPath = filepath.Join(configDirMountPathInChroot, script.Path)
	} else {
		// Write the script to a temporary file.
		tempScriptFullPath, err = createTempScriptFile(script, listName, scriptLogName, imageChroot)
		if err != nil {
			return err
		}
		defer os.Remove(tempScriptFullPath)

		// Get the path of the script file in the chroot.
		tempScriptPath, err := filepath.Rel(imageChroot.RootDir(), tempScriptFullPath)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrScriptPathResolution, err)
		}

		// Ensure path is rooted.
		tempScriptPath = filepath.Join("/", tempScriptPath)

		scriptPath = tempScriptPath
	}

	process := script.Interpreter
	if process == "" {
		process = "/bin/sh"
	}

	args := []string{scriptPath}
	args = append(args, script.Arguments...)

	envVars := []string(nil)
	for key, value := range script.EnvironmentVariables {
		envVar := fmt.Sprintf("%s=%s", key, value)
		envVars = append(envVars, envVar)
	}

	// Run the script.
	err = shell.NewExecBuilder(process, args...).
		Chroot(imageChroot.RootDir()).
		EnvironmentVariables(envVars).
		WorkingDirectory("/").
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("%w (script='%s'):\n%w", ErrScriptExecution, scriptLogName, err)
	}

	if tempScriptFullPath != "" {
		// Remove the script file and error out if the delete fails.
		err = os.Remove(tempScriptFullPath)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrScriptTempFileRemoval, err)
		}
	}

	return nil
}

func createScriptLogName(scriptIndex int, script imagecustomizerapi.Script, listName string) string {
	switch {
	case script.Name != "" && script.Path != "":
		return fmt.Sprintf("%s(%s)", script.Name, script.Path)
	case script.Name != "":
		return script.Name
	case script.Path != "":
		return script.Path
	default:
		return fmt.Sprintf("%s[%d]", listName, scriptIndex)
	}
}

func createTempScriptFile(script imagecustomizerapi.Script, listName string, scriptLogName string,
	imageChroot *safechroot.Chroot,
) (string, error) {
	chrootTempDir := filepath.Join(imageChroot.RootDir(), "tmp")

	// Create a temporary file for the script.
	tempFile, err := os.CreateTemp(chrootTempDir, listName)
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrScriptTempFileCreation, err)
	}
	defer tempFile.Close()

	tempFilePath := tempFile.Name()
	logger.Log.Debugf("Writing script's (%s) content to file (%s)", scriptLogName, tempFilePath)

	// Write the script's content.
	_, err = tempFile.WriteString(script.Content)
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrScriptTempFileWrite, err)
	}

	// Ensure the file is written correctly.
	err = tempFile.Close()
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrScriptTempFileClose, err)
	}

	return tempFilePath, nil
}
