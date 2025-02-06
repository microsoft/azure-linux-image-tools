# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import xml.etree.ElementTree as ET  # noqa: N817
import os
from pathlib import Path
from typing import Dict


class VmSpec:
    def __init__(self, name: str, memory_mib: int, core_count: int, os_disk_path: Path):
        self.name: str = name
        self.memory_mib: int = memory_mib
        self.core_count: int = core_count
        self.os_disk_path: Path = os_disk_path


# Create XML definition for a VM.
def create_libvirt_domain_xml(vm_spec: VmSpec, host_os: str, boot_type: str) -> str:
    domain = ET.Element("domain")
    domain.attrib["type"] = "kvm"

    name = ET.SubElement(domain, "name")
    name.text = vm_spec.name

    memory = ET.SubElement(domain, "memory")
    memory.attrib["unit"] = "MiB"
    memory.text = str(vm_spec.memory_mib)

    vcpu = ET.SubElement(domain, "vcpu")
    vcpu.text = str(vm_spec.core_count)

    os_tag = ET.SubElement(domain, "os")
    if boot_type == "efi" and host_os == "Ubuntu":
        os_tag.attrib["firmware"] = "efi"

    os_type = ET.SubElement(os_tag, "type")
    os_type.text = "hvm"

    if boot_type == "efi" and host_os == "Ubuntu":
        firmware = ET.SubElement(domain, "firmware")
        firmware.attrib["secure-boot"] = "yes"
        firmware.attrib["enrolled-keys"] = "yes"

    nvram = ET.SubElement(os_tag, "nvram")

    if boot_type == "efi":
        loader = ET.SubElement(os_tag, "loader")
        loader.attrib["readonly"] = "yes"
        loader.attrib["secure"] = "no"
        loader.attrib["type"] = "pflash"
        loader.text = "/usr/share/OVMF/OVMF_CODE.fd"

    features = ET.SubElement(domain, "features")

    ET.SubElement(features, "acpi")

    ET.SubElement(features, "apic")

    cpu = ET.SubElement(domain, "cpu")
    cpu.attrib["mode"] = "host-passthrough"

    clock = ET.SubElement(domain, "clock")
    clock.attrib["offset"] = "utc"

    on_poweroff = ET.SubElement(domain, "on_poweroff")
    on_poweroff.text = "destroy"

    on_reboot = ET.SubElement(domain, "on_reboot")
    on_reboot.text = "restart"

    on_crash = ET.SubElement(domain, "on_crash")
    on_crash.text = "destroy"

    devices = ET.SubElement(domain, "devices")

    serial = ET.SubElement(devices, "serial")
    serial.attrib["type"] = "pty"

    serial_target = ET.SubElement(serial, "target")
    serial_target.attrib["type"] = "isa-serial"
    serial_target.attrib["port"] = "0"

    serial_target_model = ET.SubElement(serial_target, "model")
    serial_target_model.attrib["name"] = "isa-serial"

    console = ET.SubElement(devices, "console")
    console.attrib["type"] = "pty"

    console_target = ET.SubElement(console, "target")
    console_target.attrib["type"] = "serial"
    console_target.attrib["port"] = "0"

    video = ET.SubElement(devices, "video")

    video_model = ET.SubElement(video, "model")
    video_model.attrib["type"] = "vga"

    network_interface = ET.SubElement(devices, "interface")
    network_interface.attrib["type"] = "network"

    network_interface_source = ET.SubElement(network_interface, "source")
    network_interface_source.attrib["network"] = "default"

    network_interface_model = ET.SubElement(network_interface, "model")
    network_interface_model.attrib["type"] = "virtio"

    next_disk_indexes: Dict[str, int] = {}

    _, os_disk_ext = os.path.splitext(vm_spec.os_disk_path)
    if os_disk_ext.lower() != ".iso":
        _add_disk_xml(
            devices,
            str(vm_spec.os_disk_path),
            "disk",
            "qcow2",
            "virtio",
            "vd",
            next_disk_indexes,
        )
    else:
        _add_iso_xml(
            devices,
            str(vm_spec.os_disk_path),
            "sata",
            "sd",
            next_disk_indexes,
        )

    xml = ET.tostring(domain, "unicode")
    return xml


# Adds a disk to a libvirt domain XML document.
def _add_disk_xml(
    devices: ET.Element,
    file_path: str,
    device_type: str,
    image_type: str,
    bus_type: str,
    device_prefix: str,
    next_disk_indexes: Dict[str, int],
) -> None:
    device_name = _gen_disk_device_name(device_prefix, next_disk_indexes)

    disk = ET.SubElement(devices, "disk")
    disk.attrib["type"] = "file"
    disk.attrib["device"] = device_type

    disk_driver = ET.SubElement(disk, "driver")
    disk_driver.attrib["name"] = "qemu"
    disk_driver.attrib["type"] = image_type

    disk_target = ET.SubElement(disk, "target")
    disk_target.attrib["dev"] = device_name
    disk_target.attrib["bus"] = bus_type

    disk_source = ET.SubElement(disk, "source")
    disk_source.attrib["file"] = file_path

# Adds a disk to a libvirt domain XML document.
def _add_iso_xml(
    devices: ET.Element,
    file_path: str,
    bus_type: str,
    device_prefix: str,
    next_disk_indexes: Dict[str, int],
) -> None:
    device_name = _gen_disk_device_name(device_prefix, next_disk_indexes)

    disk = ET.SubElement(devices, "disk")
    disk.attrib["type"] = "file"
    disk.attrib["device"] = "cdrom"

    disk_driver = ET.SubElement(disk, "driver")
    disk_driver.attrib["name"] = "qemu"
    disk_driver.attrib["type"] = "raw"

    disk_target = ET.SubElement(disk, "target")
    disk_target.attrib["dev"] = device_name
    disk_target.attrib["bus"] = bus_type

    disk_source = ET.SubElement(disk, "source")
    disk_source.attrib["file"] = file_path

    disk_readonly = ET.SubElement(disk, "readonly")


def _gen_disk_device_name(prefix: str, next_disk_indexes: Dict[str, int]) -> str:
    disk_index = next_disk_indexes.get(prefix, 0)
    next_disk_indexes[prefix] = disk_index + 1

    # cannot use python's 'match' because Azure Linux 2.0 has an older version
    # of python that does not support it.
    if prefix in ("vd", "sd"):
        # The disk device name is required to follow the standard Linux device naming
        # scheme. That is: [ sda, sdb, ..., sdz, sdaa, sdab, ... ]. However, it is
        # unlikely that someone will ever need more than 26 disks. So, keep it simple
        # for now.
        if disk_index < 0 or disk_index > 25:
            raise Exception(f"Unsupported disk index: {disk_index}.")
        suffix = chr(ord("a") + disk_index)
        return f"{prefix}{suffix}"
    else:
        return f"{prefix}{disk_index}"
