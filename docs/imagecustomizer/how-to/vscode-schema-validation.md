---
title: API Schema for VSCode
parent: How To
nav_order: 8
---

# Enabling VS Code Configuration Validation for Image Customizer

This guide explains how to set up YAML validation in Visual Studio Code (VS
Code) for authoring image customization configurations.

## Prerequisites

- VS Code installed on your system.

## Steps to Enable Validation

### 1. Install the YAML Extension

Download and install [YAML VS Code
extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml).
This extension provides YAML validation and syntax highlighting.

### 2. Update VS Code Settings

Modify your VS Code `settings.json` to point to the schema corresponding to the
version you are using.

Add the following to your `settings.json` file after updating `<RELEASE>` and
`<SPECIFIC-FOLDER>`:

- `<SPECIFIC-FOLDER>` is the directory containing your image configs.
- `<RELEASE>` is the version of Image Customizer that you are using (e.g.: v0.13.0).
```json
"yaml.schemas": {
    "https://raw.githubusercontent.com/microsoft/azure-linux-image-tools/refs/tags/<RELEASE>/toolkit/tools/imagecustomizerapi/schema.json": [
        "<SPECIFIC-FOLDER>/**/*.yaml"
    ]
}
```

For example:

```json
"yaml.schemas": {
    "https://raw.githubusercontent.com/microsoft/azure-linux-image-tools/refs/tags/v0.13.0/toolkit/tools/imagecustomizerapi/schema.json": [
        "/home/test/image-configs/**/*.yaml"
    ]
}
```

### 3. Validate Configurations

Once configured, YAML files in the specified directory will automatically be
validated against the schema. This enables syntax highlighting for errors and
provides instant feedback while editing your configurations. It ensures your
YAML files are properly formatted and conform to the schema.

You’re now ready to go!
