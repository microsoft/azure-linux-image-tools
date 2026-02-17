---
parent: API
grand_parent: Image Customizer
nav_order: 4
---

# Distribution Support

The following tables show which APIs are supported for each distribution.

APIs marked as **Preview** require the distribution's
[previewFeatures](./configuration/config.md#previewfeatures-string) value to be set
(e.g. `ubuntu-22.04`, `ubuntu-24.04`).

## Command line

| Subcommand                             | Azure Linux 3.0       | Ubuntu 22.04, 24.04 |
|----------------------------------------|:---------------------:|:-------------------:|
| [create](./cli/create.md)              | Yes                   | No                  |
| [convert](./cli/convert.md)            | Yes                   | Preview             |
| [customize](./cli/customize.md)        | Yes                   | Preview             |
| [inject-files](./cli/inject-files.md)  | Yes                   | No                  |

## Configuration

| API                                                                                  | Azure Linux 3.0       | Ubuntu 22.04, 24.04 |
|--------------------------------------------------------------------------------------|:---------------------:|:-------------------:|
| [input.image.path](./configuration/inputImage.md#path-string)                        | Yes                   | Preview             |
| [input.image.oci](./configuration/inputImage.md#oci-ociimage)                        | Yes                   | No                  |
| [input.image.azureLinux](./configuration/inputImage.md#azurelinux-azurelinuximage)   | Yes                   | N/A                 |
| [storage](./configuration/config.md#storage-storage)                                 | Yes                   | No                  |
| [iso](./configuration/config.md#iso-iso)                                             | Yes                   | No                  |
| [pxe](./configuration/config.md#pxe-pxe)                                             | Yes                   | No                  |
| [os.hostname](./configuration/os.md#hostname-string)                                 | Yes                   | Preview             |
| [os.kernelCommandLine](./configuration/os.md#kernelcommandline-kernelcommandline)    | Yes                   | No                  |
| [os.packages](./configuration/os.md#packages-packages)                               | Yes                   | No                  |
| [os.additionalFiles](./configuration/os.md#additionalfiles-additionalfile)           | Yes                   | Preview             |
| [os.additionalDirs](./configuration/os.md#additionaldirs-dirconfig)                  | Yes                   | Preview             |
| [os.groups](./configuration/os.md#groups-group)                                      | Yes                   | Preview             |
| [os.users](./configuration/os.md#users-user)                                         | Yes                   | Preview             |
| [os.modules](./configuration/os.md#modules-module)                                   | Yes                   | Preview             |
| [os.services](./configuration/os.md#services-services)                               | Yes                   | Preview             |
| [os.overlays](./configuration/os.md#overlays-overlay)                                | Yes                   | No                  |
| [os.bootloader](./configuration/os.md#bootloader-bootloader)                         | Yes                   | No                  |
| [os.uki](./configuration/os.md#uki-uki)                                              | Yes                   | No                  |
| [os.selinux](./configuration/os.md#selinux-selinux)                                  | Yes                   | No                  |
| [os.imageHistory](./configuration/os.md#imagehistory-string)                         | Yes                   | Preview             |
| [scripts](./configuration/config.md#scripts-scripts)                                 | Yes                   | Preview             |
| [output.image](./configuration/output.md#image-outputimage)                          | Yes                   | Preview             |
| [output.artifacts](./configuration/output.md#artifacts-outputartifacts)              | Yes                   | No                  |
| [output.selinuxPolicyPath](./configuration/output.md#selinuxpolicypath-string)       | Yes                   | No                  |
| [previewFeatures](./configuration/config.md#previewfeatures-string)                  | Yes                   | Yes                 |
