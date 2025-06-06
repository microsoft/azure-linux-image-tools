# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Azure Linux Image Creator version.
# This is using semantic versioning.
#
# IMAGE_CREATOR_VERSION should have the format:
#
#   <major>.<minor>.<patch>
#
# and should hold the value of the next (or current) official release, not the previous official
# release.
IMAGE_CREATOR_VERSION ?= 0.1.0
IMAGE_CREATOR_VERSION_PREVIEW ?= -dev.$(DATETIME_AS_VERSION)+$(GIT_COMMIT_ID)
image_CREATOR_full_version := $(IMAGE_CREATOR_VERSION)$(IMAGE_CREATOR_VERSION_PREVIEW)
