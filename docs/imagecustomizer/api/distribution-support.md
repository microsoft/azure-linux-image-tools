---
parent: API
grand_parent: Image Customizer
nav_order: 4
---

# Distribution Support

The following tables show which APIs are supported for each distribution.

APIs marked as **Preview** require the distribution's
[previewFeatures](./configuration/config.md#previewfeatures-string) value to be set
(e.g. `ubuntu-22.04`, `ubuntu-24.04`, `azure-container-linux-3`).

## Command line

| Subcommand + Flag                                                                                   | Azure Linux 3.0 | Ubuntu 22.04, 24.04 | Azure Container Linux 3.0 |
|-----------------------------------------------------------------------------------------------------|:---------------:|:-------------------:|:-------------------------:|
| [create](./cli/create.md)                                                                           | Yes             | No                  | No                        |
| [convert](./cli/convert.md)                                                                         | Yes             | Yes                 | No                        |
| [customize](./cli/customize.md)                                                                     | Yes             | Preview             | Preview                   |
| &emsp;[--build-dir](./cli/customize.md#--build-dirdirectory-path)                                   | Yes             | Preview             | Preview                   |
| &emsp;[--image-file](./cli/customize.md#--image-filefile-path)                                      | Yes             | Preview             | Preview                   |
| &emsp;[--image](./cli/customize.md#--image)                                                         | Yes             | No                  | No                        |
| &emsp;&emsp;`azureLinux:*`                                                                          | Yes             | N/A                 | N/A                       |
| &emsp;&emsp;`oci:*`                                                                                 | Yes             | Preview             | No                        |
| &emsp;[--output-image-file](./cli/customize.md#--output-image-filefile-path)                        | Yes             | Preview             | Preview                   |
| &emsp;[--output-path](./cli/customize.md#--output-pathfile-path)                                    | Yes             | Preview             | Preview                   |
| &emsp;[--output-image-format](./cli/customize.md#--output-image-formatformat)                       | Yes             | Preview             | Preview                   |
| &emsp;&emsp;`baremetal-image`                                                                       | Yes             | Preview             | No                        |
| &emsp;&emsp;`cosi`                                                                                  | Yes             | Preview             | No                        |
| &emsp;&emsp;`iso`                                                                                   | Yes             | No                  | No                        |
| &emsp;&emsp;`pxe-dir`                                                                               | Yes             | No                  | No                        |
| &emsp;&emsp;`pxe-tar`                                                                               | Yes             | No                  | No                        |
| &emsp;&emsp;`qcow2`                                                                                 | Yes             | Preview             | Preview                   |
| &emsp;&emsp;`raw`                                                                                   | Yes             | Preview             | Preview                   |
| &emsp;&emsp;`vhd-fixed`                                                                             | Yes             | Preview             | Preview                   |
| &emsp;&emsp;`vhd`                                                                                   | Yes             | Preview             | Preview                   |
| &emsp;&emsp;`vhdx`                                                                                  | Yes             | Preview             | Preview                   |
| &emsp;[--cosi-compression-level](./cli/customize.md#--cosi-compression-levellevel)                  | Yes             | No                  | No                        |
| &emsp;[--output-selinux-policy-path](./cli/customize.md#--output-selinux-policy-pathdirectory-path) | Yes             | No                  | No                        |
| &emsp;[--config-file](./cli/customize.md#--config-filefile-path)                                    | Yes             | Preview             | Preview                   |
| &emsp;[--rpm-source](./cli/customize.md#--rpm-sourcepath)                                           | Yes             | No                  | No                        |
| &emsp;[--disable-base-image-rpm-repos](./cli/customize.md#--disable-base-image-rpm-repos)           | Yes             | No                  | No                        |
| &emsp;[--package-snapshot-time](./cli/customize.md#--package-snapshot-time)                         | Yes             | No                  | No                        |
| &emsp;[--image-cache-dir](./cli/customize.md#--image-cache-dir)                                     | Yes             | No                  | No                        |
| [inject-files](./cli/inject-files.md)                                                               | Yes             | No                  | No                        |

## Configuration

| API                                                                                      | Azure Linux 3.0       | Ubuntu 22.04, 24.04 | Azure Container Linux 3.0 |
|------------------------------------------------------------------------------------------|:---------------------:|:-------------------:|:-------------------------:|
| [input.image.path](./configuration/inputImage.md#path-string)                            | Yes                   | Preview             | Preview                   |
| [input.image.oci](./configuration/inputImage.md#oci-ociimage)                            | Yes                   | No                  | No                        |
| [input.image.azureLinux](./configuration/inputImage.md#azurelinux-azurelinuximage)       | Yes                   | N/A                 | N/A                       |
| [storage](./configuration/config.md#storage-storage)                                     | Yes                   | No                  | No                        |
| [iso](./configuration/config.md#iso-iso)                                                 | Yes                   | No                  | No                        |
| [pxe](./configuration/config.md#pxe-pxe)                                                 | Yes                   | No                  | No                        |
| [os.hostname](./configuration/os.md#hostname-string)                                     | Yes                   | Preview             | Preview                   |
| [os.kernelCommandLine](./configuration/os.md#kernelcommandline-kernelcommandline)        | Yes                   | No                  | Preview                   |
| [os.packages](./configuration/os.md#packages-packages)                                   | Yes                   | Preview             | Preview                   |
| &emsp;[.updateExistingPackages](./configuration/packages.md#updateexistingpackages-bool) | Yes                   | Preview             | No                        |
| &emsp;[.installLists](./configuration/packages.md#installlists-string)                   | Yes                   | Preview             | Preview                   |
| &emsp;[.install](./configuration/packages.md#install-string)                             | Yes                   | Preview             | Preview                   |
| &emsp;[.removeLists](./configuration/packages.md#removelists-string)                     | Yes                   | Preview             | No                        |
| &emsp;[.remove](./configuration/packages.md#remove-string)                               | Yes                   | Preview             | No                        |
| &emsp;[.updateLists](./configuration/packages.md#updatelists-string)                     | Yes                   | Preview             | No                        |
| &emsp;[.update](./configuration/packages.md#update-string)                               | Yes                   | Preview             | No                        |
| &emsp;[.snapshotTime](./configuration/packages.md#snapshottime-string)                   | Yes                   | No                  | No                        |
| [os.additionalFiles](./configuration/os.md#additionalfiles-additionalfile)               | Yes                   | Preview             | Preview                   |
| [os.additionalDirs](./configuration/os.md#additionaldirs-dirconfig)                      | Yes                   | Preview             | Preview                   |
| [os.groups](./configuration/os.md#groups-group)                                          | Yes                   | Preview             | Preview                   |
| [os.users](./configuration/os.md#users-user)                                             | Yes                   | Preview             | Preview                   |
| [os.modules](./configuration/os.md#modules-module)                                       | Yes                   | Preview             | Preview                   |
| [os.services](./configuration/os.md#services-services)                                   | Yes                   | Preview             | Preview                   |
| [os.overlays](./configuration/os.md#overlays-overlay)                                    | Yes                   | No                  | No                        |
| [os.bootloader](./configuration/os.md#bootloader-bootloader)                             | Yes                   | No                  | No                        |
| [os.uki](./configuration/os.md#uki-uki)                                                  | Yes                   | No                  | Preview                   |
| [os.selinux](./configuration/os.md#selinux-selinux)                                      | Yes                   | No                  | Preview                   |
| [os.imageHistory](./configuration/os.md#imagehistory-string)                             | Yes                   | Preview             | Preview                   |
| [scripts](./configuration/config.md#scripts-scripts)                                     | Yes                   | Preview             | Preview                   |
| [output.image](./configuration/output.md#image-outputimage)                              | Yes                   | Preview             | Preview                   |
| [output.artifacts](./configuration/output.md#artifacts-outputartifacts)                  | Yes                   | No                  | Preview                   |
| [output.selinuxPolicyPath](./configuration/output.md#selinuxpolicypath-string)           | Yes                   | No                  | No                        |
| [previewFeatures](./configuration/config.md#previewfeatures-string)                      | Yes                   | Yes                 | Yes                       |
