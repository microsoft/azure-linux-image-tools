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

IMAGE_CUSTOMIZER_CONTAINER_TAG?=imagecustomizer:dev

SSH_PRIVATE_KEY_FILE ?= ${HOME}/.ssh/id_ed25519

KEEP_ENVIRONMENT ?= n

DATETIME_AS_VERSION := $(shell date +'%Y%m%d.%H%M')

.PHONY: create-venv
create-venv:
	rm -rf "${VENV_DIR}"
	python3 -m venv "${VENV_DIR}"
	${PIP} install -r requirements/dev.txt

.PHONY: update-venv
update-venv:
	${PIP} install -r requirements/dev.txt

.PHONY: check
check: mypy flake8

.PHONY: check
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

.PHONY: fix-black
fix-black:
	${BLACK} vmtests

.PHONY: ${IMAGE_CUSTOMIZER_BIN}
${IMAGE_CUSTOMIZER_BIN}:
	${MAKE} -C ${TOOLKIT_DIR} go-imagecustomizer REBUILD_TOOLS=y

.PHONY: image-customizer-container
image-customizer-container: ${IMAGE_CUSTOMIZER_BIN}
	${TOOLKIT_DIR}/tools/imagecustomizer/container/build-container.sh -t ${IMAGE_CUSTOMIZER_CONTAINER_TAG}

.PHONY: test
test:
	${PYTEST} \
		--image-customizer-container-url="${IMAGE_CUSTOMIZER_CONTAINER_TAG}" \
		--core-efi-azl2="${CORE_EFI_AZL2}" \
		--core-efi-azl3="${CORE_EFI_AZL3}" \
		--core-legacy-azl2="${CORE_LEGACY_AZL2}" \
		--core-legacy-azl3="${CORE_LEGACY_AZL3}" \
		--ssh-private-key="${SSH_PRIVATE_KEY_FILE}" \
		$(if $(filter y,$(KEEP_ENVIRONMENT)),--keep-environment) \
		--log-cli-level=DEBUG \
		--show-capture=all \
		--tb=short \
		--junitxml=./out/$(DATETIME_AS_VERSION)/report.xml \
		./vmtests

.PHONY: run
run: image-customizer-container test
