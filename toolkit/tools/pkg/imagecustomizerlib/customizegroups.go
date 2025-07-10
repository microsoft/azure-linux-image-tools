// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"strconv"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/userutils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func AddOrUpdateGroups(ctx context.Context, groups []imagecustomizerapi.Group,
	imageChroot safechroot.ChrootInterface,
) error {
	if len(groups) == 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "add_or_update_groups")
	span.SetAttributes(
		attribute.Int("groups_count", len(groups)),
	)
	defer span.End()

	for _, group := range groups {
		err := addOrUpdateGroup(group, imageChroot)
		if err != nil {
			return err
		}
	}

	return nil
}

func addOrUpdateGroup(group imagecustomizerapi.Group, imageChroot safechroot.ChrootInterface,
) error {
	// Check if the user already exists.
	groupExists, err := userutils.GroupExists(group.Name, imageChroot)
	if err != nil {
		return err
	}

	if groupExists {
		logger.Log.Infof("Updating group (%s)", group.Name)
	} else {
		logger.Log.Infof("Adding group (%s)", group.Name)
	}

	if groupExists {
		if group.GID != nil {
			return fmt.Errorf("cannot set GID (%d) on a group (%s) that already exists", *group.GID, group.Name)
		}
	} else {
		var gidStr string
		if group.GID != nil {
			gidStr = strconv.Itoa(*group.GID)
		}

		// Add the user.
		err = userutils.AddGroup(group.Name, gidStr, imageChroot)
		if err != nil {
			return err
		}
	}

	return nil
}
