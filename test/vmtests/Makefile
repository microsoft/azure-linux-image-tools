# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

VENV_DIR=build/venv
VENV_BIN_DIR=${VENV_DIR}/bin

PIP=${VENV_BIN_DIR}/pip3
PYTHON=${VENV_BIN_DIR}/python3
PYTEST=${VENV_BIN_DIR}/pytest
FLAKE8=${VENV_BIN_DIR}/flake8
BLACK=${VENV_BIN_DIR}/black
ISORT=${VENV_BIN_DIR}/isort
MYPY=${VENV_BIN_DIR}/mypy

TOOLKIT_DIR=../../toolkit
TOOLS_BIN_DIR=${TOOLKIT_DIR}/out/tools
IMAGE_CUSTOMIZER_BIN=${TOOLS_BIN_DIR}/imagecustomizer
OSMODIFIER_BIN=${TOOLS_BIN_DIR}/osmodifier

IMAGE_CUSTOMIZER_CONTAINER_TAG?=imagecustomizer:dev

SSH_PRIVATE_KEY_FILE ?= ${HOME}/.ssh/id_ed25519

KEEP_ENVIRONMENT ?= n

STOP_ON_FIRST_FAILURE ?= n

TEST_FILTER ?=

OUT_DIR ?= ./out/$(shell date +'%Y%m%d.%H%M')

LOGS_DIR := $(OUT_DIR)/logs

.PHONY: create-venv
create-venv:
	rm -rf "${VENV_DIR}"
	python3 -m venv "${VENV_DIR}"
	${PIP} install -r requirements/dev.txt

.PHONY: update-venv
update-venv:
	${PIP} install -r requirements/dev.txt

.PHONY: check
check: mypy flake8 check-black check-isort

.PHONY: flake8
flake8:
	${FLAKE8} vmtests

.PHONY: mypy
mypy:
	${MYPY} --strict --config-file pyproject.toml -p vmtests

.PHONY: fix
fix: fix-isort fix-black

.PHONY: fix-isort
fix-isort:
	${ISORT} vmtests

.PHONY: check-isort
check-isort:
	${ISORT} --check vmtests

.PHONY: fix-black
fix-black:
	${BLACK} vmtests

.PHONY: check-black
check-black:
	${BLACK} --check vmtests

.PHONY: ${IMAGE_CUSTOMIZER_BIN}
${IMAGE_CUSTOMIZER_BIN}:
	${MAKE} -C ${TOOLKIT_DIR} go-imagecustomizer

.PHONY: ${OSMODIFIER_BIN}
${OSMODIFIER_BIN}:
	${MAKE} -C ${TOOLKIT_DIR} go-osmodifier

.PHONY: image-customizer-container
image-customizer-container: ${IMAGE_CUSTOMIZER_BIN}
	${TOOLKIT_DIR}/tools/imagecustomizer/container/build-container.sh -t ${IMAGE_CUSTOMIZER_CONTAINER_TAG}

.PHONY: test-imagecustomizer
test-imagecustomizer:
	${PYTEST} \
		--image-customizer-container-url="${IMAGE_CUSTOMIZER_CONTAINER_TAG}" \
		--logs-dir="${LOGS_DIR}" \
		--core-efi-azl2="${CORE_EFI_AZL2}" \
		--core-efi-azl3="${CORE_EFI_AZL3}" \
		--core-legacy-azl2="${CORE_LEGACY_AZL2}" \
		--core-legacy-azl3="${CORE_LEGACY_AZL3}" \
		--rpm-sources-azl3="${RPM_SOURCES_AZL3}" \
		--rpm-sources-fedora42="${RPM_SOURCES_FEDORA42}" \
		--tools-file-azl3="${TOOLS_FILE_AZL3}" \
		--tools-file-fedora42="${TOOLS_FILE_FEDORA42}" \
		--ssh-private-key="${SSH_PRIVATE_KEY_FILE}" \
		$(if $(filter y,$(KEEP_ENVIRONMENT)),--keep-environment) \
		$(if $(filter y,$(STOP_ON_FIRST_FAILURE)),-x) \
		--log-cli-level=DEBUG \
		--show-capture=no \
		--tb=short \
		-vv \
		--junitxml=$(OUT_DIR)/report.xml \
		--log-file=$(OUT_DIR)/pytest.log \
		--log-file-level=DEBUG \
		-k "$(TEST_FILTER)" \
		./vmtests/imagecustomizer/test_min_change.py

.PHONY: test-osmodifier
test-osmodifier:
	${PYTEST} \
		--image-customizer-container-url="${IMAGE_CUSTOMIZER_CONTAINER_TAG}" \
		--osmodifier-binary="${OSMODIFIER_BIN}" \
		--logs-dir="${LOGS_DIR}" \
		--input-image="${INPUT_IMAGE}" \
		--ssh-private-key="${SSH_PRIVATE_KEY_FILE}" \
		$(if $(filter y,$(KEEP_ENVIRONMENT)),--keep-environment) \
		$(if $(filter y,$(STOP_ON_FIRST_FAILURE)),-x) \
		--log-cli-level=DEBUG \
		--show-capture=no \
		--tb=short \
		-vv \
		--junitxml=$(OUT_DIR)/report.xml \
		--log-file=$(OUT_DIR)/pytest.log \
		--log-file-level=DEBUG \
		-k "$(TEST_FILTER)" \
		./vmtests/osmodifier/test_osmodifier.py

.PHONY: run-imagecustomizer
run-imagecustomizer: image-customizer-container test-imagecustomizer

.PHONY: run-osmodifier
run-osmodifier: ${OSMODIFIER_BIN} test-osmodifier
