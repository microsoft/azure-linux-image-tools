# Amazon Linux 2023 vs Azure Linux Image Comparison

## Amazon Linux 2023 Published Images

### 1. **EC2 AMI Images (AWS Cloud)**
**Purpose:** Run on AWS EC2 instances
**Package Group:** AMI, AMI (minimal)

#### Available AMIs:
- **AL2023 AMI (x86_64)** - Standard EC2 image
  - URL: https://aws.amazon.com/amazon-linux-2023/
  - Download: Available through AWS Console/CLI
  - Package Group: `AMI`
  
- **AL2023 AMI Minimal (x86_64)** - Lightweight EC2 image
  - Package Group: `AMI (minimal)`
  
- **AL2023 AMI ARM64** - For Graviton processors
  - Architecture: aarch64
  
- **AL2023 AMI with Kernel 6.12** - Newer kernel variants
  - Package Groups: `AMI (kernel 6.12)`, `AMI (minimal, kernel 6.12)`

**Key Packages:**
- `amazon-ec2-net-utils`
- `amazon-linux-repo-s3`
- `cloud-init-cfg-ec2`
- `ec2-utils`
- `dracut-config-ec2`
- `hibagent`, `ec2-hibinit-agent`
- `aws-cfn-bootstrap`

#### Azure Linux Equivalent:
**Azure Linux VM Images (Azure Marketplace)**
- Published to Azure Marketplace
- Optimized for Azure VMs (Generation 1 & 2)
- Uses `cloud-init` configured for Azure
- Includes `WALinuxAgent` for Azure integration

**URLs:**
- Azure Marketplace: https://azuremarketplace.microsoft.com/
- Search: "Azure Linux" or "CBL-Mariner"
- Image SKUs: Available via `az vm image list`

---

### 2. **OnPrem Images (Hyper-V/VMware/KVM)**
**Purpose:** Run on on-premises hypervisors
**Package Group:** OnPrem, OnPrem (minimal)

#### Available OnPrem Images:

**a) Hyper-V VHD/VHDX**
- **URL:** https://cdn.amazonlinux.com/al2023/os-images/2023.9.20251117.1/
- **Format:** VHD (Generation 1), VHDX (Generation 2)
- **Example Path:** https://cdn.amazonlinux.com/al2023/os-images/2023.9.20251117.1/hyperv/
- **Package Group:** `OnPrem`

**Key Packages:**
- `amazon-linux-onprem`
- `cloud-init-cfg-onprem`
- `hyperv-daemons` (for Hyper-V integration)

#### Azure Linux Equivalent:
**Azure Linux VHD for Hyper-V**
- Format: VHD/VHDX for Hyper-V Generation 1 & 2
- Uses `hyperv-daemons` for integration services
- Cloud-init enabled for provisioning
- Available for both x86_64 and ARM64


**b) VMware OVA/VMDK**
- **URL:** https://cdn.amazonlinux.com/al2023/os-images/
- **Format:** OVA, VMDK
- **Example:** https://cdn.amazonlinux.com/al2023/os-images/2023.9.20251117.1/vmware/
- **Package Group:** `OnPrem`

#### Azure Linux Equivalent:
**Azure Linux OVA for VMware**
- Available in Azure Linux releases
- ESXi/vSphere compatible
- Uses `open-vm-tools`

**c) KVM/libvirt QCOW2**
- **URL:** https://cdn.amazonlinux.com/al2023/os-images/
- **Format:** QCOW2
- **Example:** https://cdn.amazonlinux.com/al2023/os-images/2023.9.20251117.1/kvm/
- **Package Group:** `OnPrem`

#### Azure Linux Equivalent:
**Azure Linux QCOW2 for KVM**
- Available in Azure Linux
- Compatible with libvirt/QEMU/KVM

---

### 3. **Container Images**
**Purpose:** Run in Docker/Podman
**Package Group:** Container

#### Available Container Images:

**Amazon ECR Public**
- **URL:** https://gallery.ecr.aws/amazonlinux/amazonlinux
- **Image:** `public.ecr.aws/amazonlinux/amazonlinux:2023`
- **Pull:** `docker pull public.ecr.aws/amazonlinux/amazonlinux:2023`

**Key Packages (Minimal):**
- `coreutils-single` (not full coreutils)
- `curl-minimal`
- `glibc-minimal-langpack`
- `gnupg2-minimal`

#### Azure Linux Equivalent:
**Azure Linux Container Images**
- **MCR (Microsoft Container Registry):** `mcr.microsoft.com/cbl-mariner/base/core:2.0`
- **Pull:** `docker pull mcr.microsoft.com/cbl-mariner/base/core:2.0`

---

### 4. **ISO Images (Installation Media)**
**Purpose:** Bare metal installation

#### Available ISOs:
- **URL:** https://cdn.amazonlinux.com/al2023/os-images/
- **Format:** ISO
- **Type:** Installation ISO for bare metal

#### Azure Linux Equivalent:
**Azure Linux ISO**
- Available in Azure Linux releases
- For bare metal/VM installation

---

## Summary Mapping Table

| Amazon Linux 2023 | Azure Linux Equivalent | Use Case |
|-------------------|------------------------|----------|
| **AMI** | Azure VM Marketplace Image | Cloud VMs (AWS EC2 ↔ Azure VM) |
| **AMI (minimal)** | Azure Linux Core Image? | Minimal cloud VMs |
| **OnPrem Hyper-V VHD** | Azure Linux hyper-v VHD/VHDX | Hyper-V virtualization |
| **OnPrem VMware OVA** | Azure Linux OVA | VMware ESXi/vSphere |
| **OnPrem KVM QCOW2** | Azure Linux qemu QCOW2 | KVM/libvirt |
| **Container (Docker)** | CBL-Mariner Container | Containers |
| **ISO** | Azure Linux ISO /bare metal image| Bare metal install |

---
