# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

######## MISC. MAKEFILE Functions ########

# Creates a folder if it doesn't exist. Also sets the timestamp to 0 if it is
# created.
#
# $1 - Folder path
define create_folder
$(call shell_real_build_only, if [ ! -d $1 ]; then mkdir -p $1 && touch -d @0 $1 ; fi )
endef

# Runs a shell command only if we are actually doing a build rather than parsing the makefile for tab-completion etc
# Make will automatically create the MAKEFLAGS variable which contains each of the flags, non-build commmands will include -n
# which is the short form of --dry-run.
#
# $1 - The full command to run, if we are not doing --dry-run
ifeq (n,$(findstring n,$(firstword $(MAKEFLAGS))))
shell_real_build_only =
else # ifeq (n,$(findstring...
shell_real_build_only = $(shell $1)
endif # ifeq (n,$(findstring...

# Echos a message to console, then calls "exit 1"
# Of the form: { echo "MSG" ; exit 1 ; }
#
# $1 - Error message to print
define print_error
{ echo "$1" ; exit 1 ; }
endef

# Echos a message to console, then, if STOP_ON_WARNING is set to "y" calls "exit 1"
# Of the form: { echo "MSG" ; < exit 1 ;> }
#
# $1 - Warning message to print
define print_warning
{ echo "$1" ; $(if $(filter y,$(STOP_ON_WARNING)),exit 1 ;) }
endef

######## VARIABLE DEPENDENCY TRACKING ########

.PHONY: variable_depends_on_phony clean-variable_depends_on_phony setfacl_always_run_phony
clean: clean-variable_depends_on_phony

$(call create_folder,$(STATUS_FLAGS_DIR))
clean-variable_depends_on_phony:
	rm -rf $(STATUS_FLAGS_DIR)
