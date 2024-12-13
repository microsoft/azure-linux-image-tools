# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

toolkit_root := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SCRIPTS_DIR  ?= $(toolkit_root)/scripts

# Folder defines
TOOLS_DIR        ?= $(toolkit_root)/tools
TOOL_BINS_DIR    ?= $(toolkit_root)/out/tools
RESOURCES_DIR    ?= $(toolkit_root)/resources

PROJECT_ROOT     ?= $(realpath $(toolkit_root)/..)
BUILD_DIR        ?= $(PROJECT_ROOT)/build
STATUS_FLAGS_DIR   ?= $(BUILD_DIR)/make_status

include $(SCRIPTS_DIR)/utils.mk

LOG_LEVEL          ?= info
LOG_COLOR          ?= auto

# Set the variables for build number, distro tag, etc
include $(SCRIPTS_DIR)/build_tag.mk
include $(SCRIPTS_DIR)/build_tag_imagecustomizer.mk

include $(SCRIPTS_DIR)/tools.mk

clean:
	rm -rf $(toolkit_root)/out

######## VARIABLE PRINTING ########

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
