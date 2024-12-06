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

.PHONY: run
run:
	${PYTEST} \
		--log-cli-level=DEBUG \
		--show-capture=all \
		--tb=short \
		./vmtests
