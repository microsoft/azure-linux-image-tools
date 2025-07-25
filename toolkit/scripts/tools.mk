# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Contains:
#	- Go Utilities

$(call create_folder,$(CCACHE_DIR))
$(call create_folder,$(TOOL_BINS_DIR))
$(call create_folder,$(BUILD_DIR)/tools)

######## GO TOOLS ########

# Scans and include licenses for all the go tools
.PHONY: license-scan
license-scan:
	./scripts/check-and-collect-licenses.py

# The version as held in the go.mod file (a line like 'go 1.19'). Add "go" to the front of the version number
# so that it matches the output of 'go version' (e.g. 'go1.19').
go_min_version = go$(shell grep -E '^go [0-9]+\.[0-9]+' $(TOOLS_DIR)/go.mod | awk '{print $$2}')

# Check if the go version is high enough to build the tools. The 'sort' command is used to compare the versions
# (with -V which sorts by version number). If the lowest version in the sort is the same as the minimum version, then
# the installed version must be greater than or equal to the minimum version and we are fine.
go_current_version = $(shell go version | awk '{print $$3}')
go_version_check = $(shell printf '%s\n%s\n' "$(go_min_version)" "$(go_current_version)" | sort -V | head -n1)
ifneq ($(go_version_check),$(go_min_version))
$(error Go version '$(go_current_version)' is less than minimum required version '$(go_min_version)')
endif

# List of go utilities in tools/ directory
go_tool_list = \
	imagecustomizer \
	imager \
	isomaker \
	osmodifier \
	imagecreator \

# For each utility "util", create a "out/tools/util" target which references code in "tools/util/"
go_tool_targets = $(foreach target,$(go_tool_list),$(TOOL_BINS_DIR)/$(target))
# Common files to monitor for all go targets
go_module_files = $(TOOLS_DIR)/go.mod $(TOOLS_DIR)/go.sum
go_internal_files = $(shell find $(TOOLS_DIR)/internal/ -type f -name '*.go')
go_pkg_files = $(shell find $(TOOLS_DIR)/pkg/ -type f -name '*.go')
go_common_files = $(go_module_files) $(go_internal_files) $(go_pkg_files) $(go_scheduler_files) $(STATUS_FLAGS_DIR)/got_go_deps.flag $(BUILD_DIR)/tools/internal.test_coverage


# A report on test coverage for all the go tools
test_coverage_report=$(TOOL_BINS_DIR)/test_coverage_report.html

# For each utility "util", create an alias variable "$(go-util)", and a target "go-util".
# Also add file dependencies for the various tools.
#	go-util=$(TOOL_BINS_DIR)/util
#	.PHONY: go-util
#	go-util: $(go-util)
#   $(TOOL_BINS_DIR)/util: $(TOOLS_DIR)/util/*.go
define go_util_rule
go-$(notdir $(tool))=$(tool)
.PHONY: go-$(notdir $(tool))
go-$(notdir $(tool)): $(tool)
$(tool): $(call shell_real_build_only, find $(TOOLS_DIR)/$(notdir $(tool))/ -type f -name '*.go')
endef
$(foreach tool,$(go_tool_targets),$(eval $(go_util_rule)))

.PHONY: go-tools clean-go-tools go-tidy-all go-test-coverage
##help:target:go-tools=Preps all go tools.
go-tools: $(go_tool_targets)

clean: clean-go-tools
clean-go-tools:
	rm -rf $(TOOL_BINS_DIR)
	rm -rf $(BUILD_DIR)/tools

go_ldflags := 	-X github.com/microsoft/azurelinux/toolkit/tools/internal/exe.ToolkitVersion=$(RELEASE_VERSION) \
				-X github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib.ToolVersion=$(image_customizer_full_version) \
				-X github.com/microsoft/azurelinux/toolkit/tools/internal/exe.DistroNameAbbreviation=$(DIST_NAME_ABRV) \
				-X github.com/microsoft/azurelinux/toolkit/tools/internal/exe.DistroMajorVersion=$(dist_major_version_number)

# Conditionally include license-scan dependency based on SKIP_LICENSE_SCAN flag
ifeq ($(SKIP_LICENSE_SCAN),y)
license_scan_dependency =
else
license_scan_dependency = license-scan
endif

# Matching rules for the above targets
# Tool specific pre-requisites are tracked via $(go-util): $(shell find...) dynamic variable defined above
$(TOOL_BINS_DIR)/%: $(go_common_files) $(license_scan_dependency)
	cd $(TOOLS_DIR)/$* && \
		go test -ldflags="$(go_ldflags)" -test.short -covermode=atomic -coverprofile=$(BUILD_DIR)/tools/$*.test_coverage ./... && \
		CGO_ENABLED=0 go build \
			-ldflags="$(go_ldflags)" \
			$(if $(filter y,$(BUILD_TOOLS_NONPROD)),,-tags prod) \
			-o $(TOOL_BINS_DIR)

# Runs tests for common components
$(BUILD_DIR)/tools/internal.test_coverage: $(go_internal_files) $(go_imagegen_files) $(STATUS_FLAGS_DIR)/got_go_deps.flag
	cd $(TOOLS_DIR)/$* && \
		go test -ldflags="$(go_ldflags)" -test.short -covermode=atomic -coverprofile=$@ ./...

.PHONY: imagecustomizer-targz
imagecustomizer-targz: go-imagecustomizer $(license_scan_dependency)
	rm -rf $(BUILD_DIR)/imagecustomizertar || true
	mkdir -p $(BUILD_DIR)/imagecustomizertar
	cp $(TOOL_BINS_DIR)/imagecustomizer $(BUILD_DIR)/imagecustomizertar
ifeq ($(SKIP_LICENSE_SCAN),y)
	@echo "Skipping license scan - LICENSES directory will not be included"
else
	cp -r $(toolkit_root)/out/LICENSES $(BUILD_DIR)/imagecustomizertar
endif
	tar -C $(BUILD_DIR)/imagecustomizertar -cz --file $(toolkit_root)/out/imagecustomizer.tar.gz .

.PHONY: imagecreator-targz
imagecreator-targz: go-imagecreator $(license_scan_dependency)
	rm -rf $(BUILD_DIR)/imagecreatortargz || true
	mkdir -p $(BUILD_DIR)/imagecreatortargz
	cp $(TOOL_BINS_DIR)/imagecreator $(BUILD_DIR)/imagecreatortargz
ifeq ($(SKIP_LICENSE_SCAN),y)
	@echo "Skipping license scan - LICENSES directory will not be included"
else
	cp -r $(toolkit_root)/out/LICENSES $(BUILD_DIR)/imagecreatortargz
endif
	tar -C $(BUILD_DIR)/imagecreatortargz -cz --file $(toolkit_root)/out/imagecreator.tar.gz .

# Downloads all the go dependencies without using sudo, so we don't break other go use cases for the user.
# We can check if $SUDO_USER is set (the user who invoked sudo), and if so, use that user to run go get via sudo -u.
# We allow the command to fail with || echo ..., since we don't want to fail the build if the user has already
# downloaded the dependencies as root. The go build command will download the dependencies if they are missing (but as root).
$(STATUS_FLAGS_DIR)/got_go_deps.flag:
	@cd $(TOOLS_DIR)/ && \
		if [ -z "$$SUDO_USER" ]; then \
			echo "SUDO_USER is not set, running 'go get' as user '$$USER'"; \
			go get -d ./... || echo "Failed to run 'go get', falling back to 'go build' to pull modules" ; \
		else \
			echo "SUDO_USER is set, running 'go get' as user '$$SUDO_USER'"; \
			sudo -u $$SUDO_USER go get -d ./... || echo "Failed to run 'go get', falling back to 'go build' to pull modules" ; \
		fi && \
		touch $@

##help:target:go-tidy-all=Runs `go-fmt-all` and `go-mod-tidy`.
# Return a list of all directories inside tools/ which contains a *.go file in
# the form of "go-fmt-<directory>"
go-tidy-all: go-mod-tidy go-fmt-all
##help:target:go-mod-tidy=Tidy the go module files.
# Updates the go module file
go-mod-tidy:
	rm -f $(TOOLS_DIR)/go.sum
	cd $(TOOLS_DIR) && go mod tidy
##help:target:go-fmt-all=Auto format all `*.go` files.
# Runs go fmt inside each matching directory
go-fmt-all:
	cd $(TOOLS_DIR) && go fmt ./...

# Formats the test coverage for the tools
.PHONY: $(BUILD_DIR)/tools/all_tools.coverage
$(BUILD_DIR)/tools/all_tools.coverage: $(call shell_real_build_only, find $(TOOLS_DIR)/ -type f -name '*.go') $(STATUS_FLAGS_DIR)/got_go_deps.flag
	cd $(TOOLS_DIR) && go test -ldflags="$(go_ldflags)" -coverpkg=./... -test.short -covermode=atomic -coverprofile=$@ ./...
$(test_coverage_report): $(BUILD_DIR)/tools/all_tools.coverage
	cd $(TOOLS_DIR) && go tool cover -html=$(BUILD_DIR)/tools/all_tools.coverage -o $@
##help:target:go-test-coverage=Run and publish test coverage for all go tools.
go-test-coverage: $(test_coverage_report)
	@echo Coverage report available at: $(test_coverage_report)
