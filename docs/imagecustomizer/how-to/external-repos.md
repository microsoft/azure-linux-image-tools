---
title: Install/Update Packages from External Repos
parent: How To
nav_order: 7
---

# Install/Update Packages from External Repos

This guide explains how to install or update packages from external repositories
using Image Customizer. The example below demonstrates installing Kubernetes
(k8s) packages from the `cloud-native` repository.

## Steps

1. Download the Base Image

   Have the image under customization ready. For using marketplace image, see
   [Downloading marketplace image](./azure-vm/download-marketplace-image.md)

2. Prepare the Repository Configuration

   Create a repository configuration file (e.g. `cloud-native-prod.repo`). The
   following example is based on the [cloud-native
   repo](https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/config.repo).
   You can also set up your own repo, See [Clone rpm repo](../reference/clone-rpm-repo.md)
   for details.

   ```ini
   [azurelinux3.0prodcloud-nativex86_64]
   name=Azure Linux 3.0 Cloud-Native Repo (x86_64)
   baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/
   gpgcheck=1
   repo_gpgcheck=1
   enabled=1
   gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
   sslverify=1
   ```

3. Update the Customization Configuration:

   Specify the packages to be installed in your Image Customizer configuration
   file:

   ```yaml
   os:
     packages:
        install:
        - kubeadm
        - kubectl
        - kubelet
   ```

4. Persist the Repository for Runtime Updates

   To enable package updates at runtime, copy the repository configuration to
   `/etc/yum.repos.d/`

   ```yaml
   os:
     additionalFiles:
     - source: repos/cloud-native-prod.repo
       destination: /etc/yum.repos.d/cloud-native-prod.repo
   ```

5. Run Image Customizer with the Repository Configuration

   Execute Image Customizer, passing the repository configuration via
   `--rpm-source`:

   Note: Passing multiple `--rpm-source` entries is supported.

   ```bash
   sudo ./imagecustomizer \
         --build-dir ./build \
         --image-file <base-image-file> \
         --output-image-file ./out/image.vhdx \
         --output-image-format vhdx \
         --config-file <config-file> \
         --rpm-source repos/cloud-native-prod.repo
   ```
