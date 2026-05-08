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

| Subcommand + Flag                                                                                   | Azure Linux 3.0 | Azure Linux 4.0 | Ubuntu 22.04, 24.04 |
|-----------------------------------------------------------------------------------------------------|:---------------:|:---------------:|:-------------------:|
| [create](./cli/create.md)                                                                           | Yes             | Yes             | No                  |
| [convert](./cli/convert.md)                                                                         | Yes             | Yes             | Yes                 |
| [customize](./cli/customize.md)                                                                     | Yes             | Yes             | Preview             |
| &emsp;[--build-dir](./cli/customize.md#--build-dirdirectory-path)                                   | Yes             | Yes             | Preview             |
| &emsp;[--image-file](./cli/customize.md#--image-filefile-path)                                      | Yes             | Yes             | Preview             |
| &emsp;[--image](./cli/customize.md#--image)                                                         | Yes             | Yes             | No                  |
| &emsp;&emsp;`azureLinux:*`                                                                          | Yes             | Yes             | N/A                 |
| &emsp;&emsp;`oci:*`                                                                                 | Yes             | Yes             | Preview             |
| &emsp;[--output-image-file](./cli/customize.md#--output-image-filefile-path)                        | Yes             | Yes             | Preview             |
| &emsp;[--output-path](./cli/customize.md#--output-pathfile-path)                                    | Yes             | Yes             | Preview             |
| &emsp;[--output-image-format](./cli/customize.md#--output-image-formatformat)                       | Yes             | Yes             | Preview             |
| &emsp;&emsp;`baremetal-image`                                                                       | Yes             | No              | Preview             |
| &emsp;&emsp;`cosi`                                                                                  | Yes             | No              | Preview             |
| &emsp;&emsp;`iso`                                                                                   | Yes             | No              | No                  |
| &emsp;&emsp;`pxe-dir`                                                                               | Yes             | No              | No                  |
| &emsp;&emsp;`pxe-tar`                                                                               | Yes             | No              | No                  |
| &emsp;&emsp;`qcow2`                                                                                 | Yes             | Yes             | Preview             |
| &emsp;&emsp;`raw`                                                                                   | Yes             | Yes             | Preview             |
| &emsp;&emsp;`vhd-fixed`                                                                             | Yes             | Yes             | Preview             |
| &emsp;&emsp;`vhd`                                                                                   | Yes             | Yes             | Preview             |
| &emsp;&emsp;`vhdx`                                                                                  | Yes             | Yes             | Preview             |
| &emsp;[--cosi-compression-level](./cli/customize.md#--cosi-compression-levellevel)                  | Yes             | No              | No                  |
| &emsp;[--output-selinux-policy-path](./cli/customize.md#--output-selinux-policy-pathdirectory-path) | Yes             | No              | No                  |
| &emsp;[--config-file](./cli/customize.md#--config-filefile-path)                                    | Yes             | Yes             | Preview             |
| &emsp;[--rpm-source](./cli/customize.md#--rpm-sourcepath)                                           | Yes             | Yes             | No                  |
| &emsp;[--disable-base-image-rpm-repos](./cli/customize.md#--disable-base-image-rpm-repos)           | Yes             | Yes             | No                  |
| &emsp;[--package-snapshot-time](./cli/customize.md#--package-snapshot-time)                         | Yes             | No              | No                  |
| &emsp;[--image-cache-dir](./cli/customize.md#--image-cache-dir)                                     | Yes             | Yes             | No                  |
| [inject-files](./cli/inject-files.md)                                                               | Yes             | Yes             | No                  |

## Configuration

| API                                                                                      | Azure Linux 3.0       | Azure Linux 4.0       | Ubuntu 22.04, 24.04 |
|------------------------------------------------------------------------------------------|:---------------------:|:---------------------:|:-------------------:|
| [input.image.path](./configuration/inputImage.md#path-string)                            | Yes                   | Yes                   | Preview             |
| [input.image.oci](./configuration/inputImage.md#oci-ociimage)                            | Yes                   | Yes                   | No                  |
| [input.image.azureLinux](./configuration/inputImage.md#azurelinux-azurelinuximage)       | Yes                   | Yes                   | N/A                 |
| [storage](./configuration/config.md#storage-storage)                                     | Yes                   | No                    | No                  |
| [iso](./configuration/config.md#iso-iso)                                                 | Yes                   | No                    | No                  |
| [pxe](./configuration/config.md#pxe-pxe)                                                 | Yes                   | No                    | No                  |
| [os.hostname](./configuration/os.md#hostname-string)                                     | Yes                   | Yes                   | Preview             |
| [os.kernelCommandLine](./configuration/os.md#kernelcommandline-kernelcommandline)        | Yes                   | No                    | No                  |
| [os.packages](./configuration/os.md#packages-packages)                                   | Yes                   | Yes                   | Preview             |
| &emsp;[.updateExistingPackages](./configuration/packages.md#updateexistingpackages-bool) | Yes                   | Yes                   | Preview             |
| &emsp;[.installLists](./configuration/packages.md#installlists-string)                   | Yes                   | Yes                   | Preview             |
| &emsp;[.install](./configuration/packages.md#install-string)                             | Yes                   | Yes                   | Preview             |
| &emsp;[.removeLists](./configuration/packages.md#removelists-string)                     | Yes                   | Yes                   | Preview             |
| &emsp;[.remove](./configuration/packages.md#remove-string)                               | Yes                   | Yes                   | Preview             |
| &emsp;[.updateLists](./configuration/packages.md#updatelists-string)                     | Yes                   | Yes                   | Preview             |
| &emsp;[.update](./configuration/packages.md#update-string)                               | Yes                   | Yes                   | Preview             |
| &emsp;[.snapshotTime](./configuration/packages.md#snapshottime-string)                   | Yes                   | No                    | No                  |
| [os.additionalFiles](./configuration/os.md#additionalfiles-additionalfile)               | Yes                   | Yes                   | Preview             |
| [os.additionalDirs](./configuration/os.md#additionaldirs-dirconfig)                      | Yes                   | Yes                   | Preview             |
| [os.groups](./configuration/os.md#groups-group)                                          | Yes                   | Yes                   | Preview             |
| [os.users](./configuration/os.md#users-user)                                             | Yes                   | Yes                   | Preview             |
| [os.modules](./configuration/os.md#modules-module)                                       | Yes                   | Yes                   | Preview             |
| [os.services](./configuration/os.md#services-services)                                   | Yes                   | Yes                   | Preview             |
| [os.overlays](./configuration/os.md#overlays-overlay)                                    | Yes                   | Yes                   | No                  |
| [os.bootloader](./configuration/os.md#bootloader-bootloader)                             | Yes                   | No                    | No                  |
| [os.uki](./configuration/os.md#uki-uki)                                                  | Yes                   | No                    | No                  |
| [os.selinux](./configuration/os.md#selinux-selinux)                                      | Yes                   | No                    | No                  |
| [os.imageHistory](./configuration/os.md#imagehistory-string)                             | Yes                   | Yes                   | Preview             |
| [scripts](./configuration/config.md#scripts-scripts)                                     | Yes                   | Yes                   | Preview             |
| [output.image](./configuration/output.md#image-outputimage)                              | Yes                   | Yes                   | Preview             |
| [output.artifacts](./configuration/output.md#artifacts-outputartifacts)                  | Yes                   | No                    | No                  |
| [output.selinuxPolicyPath](./configuration/output.md#selinuxpolicypath-string)           | Yes                   | No                    | No                  |
| [previewFeatures](./configuration/config.md#previewfeatures-string)                      | Yes                   | Yes                   | Yes                 |
