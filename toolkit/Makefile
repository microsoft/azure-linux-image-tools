# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Contains:
#   - Definitions
#   - High Level Targets
#   - Submake Includes

######## DEFINITIONS ########

toolkit_root := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SCRIPTS_DIR  ?= $(toolkit_root)/scripts

######## SET REMAINING FLAGS ########

RUN_CHECK                            ?= n

##help:var:REPO_SNAPSHOT_TIME:<posix_time>=Posix time to be used as a snapshot for remote repositories when fetching packages. Example: REPO_SNAPSHOT_TIME="1724119509".
CACHE_DIR                            ?=
PACKAGE_CACHE_SUMMARY                ?=
IMAGE_CACHE_SUMMARY                  ?=
INITRD_CACHE_SUMMARY                 ?=
PACKAGE_ARCHIVE                      ?=
PACKAGE_BUILD_RETRIES                ?= 0
CHECK_BUILD_RETRIES                  ?= 0
EXTRA_BUILD_LAYERS                   ?= 0
REFRESH_WORKER_CHROOT                ?= y
TARGET_ARCH                          ?=
ALLOW_TOOLCHAIN_REBUILDS             ?= n
CACHED_PACKAGES_ARCHIVE              ?=
USE_CCACHE                           ?= n

# Folder defines
TOOLS_DIR        ?= $(toolkit_root)/tools
TOOL_BINS_DIR    ?= $(toolkit_root)/out/tools
RESOURCES_DIR    ?= $(toolkit_root)/resources

PROJECT_ROOT     ?= $(realpath $(toolkit_root)/..)
BUILD_DIR        ?= $(PROJECT_ROOT)/build
OUT_DIR          ?= $(PROJECT_ROOT)/out
CCACHE_DIR       ?= $(PROJECT_ROOT)/ccache
CCACHE_CONFIG    ?= $(RESOURCES_DIR)/manifests/package/ccache-configuration.json

# Sub-folder defines
LOGS_DIR           ?= $(BUILD_DIR)/logs
CACHED_RPMS_DIR    ?= $(BUILD_DIR)/rpm_cache
STATUS_FLAGS_DIR   ?= $(BUILD_DIR)/make_status
CHROOT_DIR         ?= $(BUILD_DIR)/worker/chroot
IMAGEGEN_DIR       ?= $(BUILD_DIR)/imagegen
PROFILE_DIR        ?= $(BUILD_DIR)/profile

META_USER_DATA_DIR ?= $(RESOURCES_DIR)/assets/meta-user-data
SSH_KEY_FILE       ?=

IMAGES_DIR      ?= $(OUT_DIR)/images

######## COMMON MAKEFILE UTILITIES ########

# Misc function defines
# Variable prerequisite tracking
include $(SCRIPTS_DIR)/utils.mk

######## REMAINING BUILD FLAGS ########

##help:var:LOG_LEVEL:{panic,fatal,error,warn,info,debug,trace}=Set logging level for toolkit.
# panic,fatal,error,warn,info,debug,trace
LOG_LEVEL          ?= info
##help:var:LOG_COLOR:{always,auto,never}=Set logging color for toolkit terminal output.
# always,auto,never
LOG_COLOR          ?= auto
STOP_ON_WARNING    ?= n
STOP_ON_PKG_FAIL   ?= n
STOP_ON_FETCH_FAIL ?= n

######## HIGH LEVEL TARGETS ########

.PHONY: all clean
all: toolchain go-tools chroot-tools

######## SUBMAKE INCLUDES ########

# General help information used by 'help' target; this should be included first so
# its help will be displayed at the top of the help output.
include $(SCRIPTS_DIR)/help.mk

# Set the variables for build number, distro tag, etc
include $(SCRIPTS_DIR)/build_tag.mk
include $(SCRIPTS_DIR)/build_tag_imagecustomizer.mk

# go utilities with:
#   go-tools, clean-go-tools, go-tidy-all (tidy go utilities before committing) go-test-coverage
include $(SCRIPTS_DIR)/tools.mk

# chroot worker with:
#   chroot-tools clean-chroot-tools
# macro definitions with:
#   macro-tools clean-macro-tools
include $(SCRIPTS_DIR)/chroot.mk

# Create images with:
#   image, iso, clean-imagegen
include $(SCRIPTS_DIR)/imggen.mk

##help:target:clean=Clean all built files.
# Each component provides specific clean implementations which clean depends on.
# They are guaranteed to run first and will verify there are no existing mount points
# left after a chroot.
clean:
	rm -rf $(OUT_DIR)
	rm -rf $(BUILD_DIR)
	rm -rf $(toolkit_root)/out

######## VARIABLE PRINTING ########

# Display Makefile usage help; please consult $(SCRIPTS_DIR)/help.mk for documentation
# on the format of the comment annotations being extracted here.
.PHONY: help
help:
	@sed -ne '/@sed/!s/##help:desc://p' $(MAKEFILE_LIST)
	@sed -ne '/@sed/!s/##help:target-heading://p' $(MAKEFILE_LIST)
	@grep -h "^##help:target:" $(MAKEFILE_LIST) | sed -e "s/^##help:target:\([^=]\+\)=\(.*\)/\1|\2/" | sort | awk -F'|' '{ printf("  \033[3m%-26s\033[0m %s\n", $$1, $$2)}'
	@sed -ne '/@sed/!s/##help:var-heading://p' $(MAKEFILE_LIST)
	@grep -h "^##help:var:" $(MAKEFILE_LIST) | sed -e "s/^##help:var:\([^:]\+\):\([^=]*\)=\(.*\)/\1|\2|\3/" | sort | awk -F'|' '{ printf("  \033[92;3m%s\033[0m=%s\n      %s\n\n", $$1, $$2, $$3) }'
	@sed -ne '/@sed/!s/##help:example-heading://p' $(MAKEFILE_LIST)
	@grep -h "^##help:example:" $(MAKEFILE_LIST) | sed -e "s/^##help:example:\(.*\)/  \1/"

# Some common version information that is useful to gather. Generally should be run with the Make flag --quiet
get-version: printvar-RELEASE_VERSION
get-dist-tag: printvar-DIST_TAG
get-release-major: printvar-RELEASE_MAJOR_ID

# Make an easy way to print out the build variables. These must be the last entries in the makefile so that all other
# files have their variables inluded

# Print out all variables to stdout, either or of the form "<VALUE>" or the verbose form "varname: <VALUE>"
#   printvar-all-vars
#   printvar-verbose-all-vars

# Print a specific variable to stdout, using the same format as above
#   printvar-*
#   printvar-verbose-*

# Use these targets like so:  `my-var=$(make printvar-MY_VAR --quiet)`
# The --quiet flag is important to avoid printing extra output
.PHONY: printvar-all-vars printvar-verbose-all-vars

# Gather the variables we want to print out, removing any automatic .* variables, and the self reference, along with special characters that may interfere with Make
sanitize_variables = $(subst ',,$(subst ",,$(subst `,,$(subst \#,,$(subst $$,,$(subst :,,$1))))))
interesting_variables  = $(filter-out .% interesting_variables, $(sort $(call sanitize_variables,$(.VARIABLES))))

printvar-all-vars: $(foreach var,$(interesting_variables),printvar-$(var))
printvar-all-vars: ;
printvar-verbose-all-vars: $(foreach var,$(interesting_variables),printvar-verbose-$(var))
printvar-verbose-all-vars: ;

printvar-%: ; $(info $($(subst printvar-,,$@)))
	@: # We want to supress 'make: Nothing to be done for ...' so execute a command so make thinks it has done something
printvar-verbose-%: ; $(info $(subst printvar-verbose-,,$@): $($(subst printvar-verbose-,,$@)))
	@: # We want to supress 'make: Nothing to be done for ...' so execute a command so make thinks it has done something
