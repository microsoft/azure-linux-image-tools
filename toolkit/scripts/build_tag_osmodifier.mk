# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Azure Linux OSModifier version.
# This is using semantic versioning.
#
# OSMODIFIER_VERSION should have the format:
#
#   <major>.<minor>.<patch>
#
# and should hold the value of the next (or current) official release, not the previous official
# release.
OSMODIFIER_VERSION ?= 0.19.0
OSMODIFIER_VERSION_PREVIEW ?= -dev.$(DATETIME_AS_VERSION)+$(GIT_COMMIT_ID)
osmodifier_full_version := $(OSMODIFIER_VERSION)$(OSMODIFIER_VERSION_PREVIEW)
