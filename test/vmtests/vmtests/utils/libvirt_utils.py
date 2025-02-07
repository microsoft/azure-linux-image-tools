# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import fnmatch
import logging
import json
import xml.etree.ElementTree as ET  # noqa: N817
import libvirt  # type: ignore
from . import local_client
import os
from pathlib import Path
from typing import Dict, List, Any


class VmSpec:
    def __init__(self, name: str, memory_mib: int, core_count: int, os_disk_path: Path):
        self.name: str = name
        self.memory_mib: int = memory_mib
        self.core_count: int = core_count
        self.os_disk_path: Path = os_disk_path

# Read a bunch of JSON files that have been concatenated together.
def _read_concat_json_str(json_str: str) -> List[Dict[str, Any]]:
    objs = []

    # From: https://stackoverflow.com/a/42985887
    decoder = json.JSONDecoder()
    text = json_str.lstrip()  # decode hates leading whitespace
    while text:
        obj, index = decoder.raw_decode(text)
        text = text[index:].lstrip()

        objs.append(obj)

    return objs
    
def get_machine_type(
    libvirt_conn: libvirt.virConnect,
) -> str:
    domain_capabilities_str = libvirt_conn.getDomainCapabilities(None, None, None, None, 0)
    domain_capabilities = ET.fromstring(domain_capabilities_str)
    return domain_capabilities.find("./machine").text

def get_firmware_config(
        libvirt_conn: libvirt.virConnect,
        machine_type: str,
        enable_secure_boot: bool,
    ) -> Dict[str, Any]:
        # Resolve the machine type to its full name.
        domain_caps_str = libvirt_conn.getDomainCapabilities(
            machine=machine_type, virttype="kvm"
        )
        domain_caps = ET.fromstring(domain_caps_str)

        full_machine_type = domain_caps.findall("./machine")[0].text
        arch = domain_caps.findall("./arch")[0].text

        # Read the QEMU firmware config files.
        # Note: "/usr/share/qemu/firmware" is a well known location for these files.
        # Loop through all .json files in the folder
        firmware_configs_str=""
        firmware_definitions_path = Path("/usr/share/qemu/firmware")  # Change to your folder path
        for firmware_definition_file in firmware_definitions_path.glob("*.json"):
            try:
                with firmware_definition_file.open("r", encoding="utf-8") as f:
                    data = f.read()
                    firmware_configs_str += data
            except json.JSONDecodeError as e:
                print(f"Error reading {firmware_definition_file.name}: {e}")

        firmware_configs = _read_concat_json_str(firmware_configs_str)

        # Filter on architecture.
        filtered_firmware_configs = list(
            filter(lambda f: f["targets"][0]["architecture"] == arch, firmware_configs)
        )

        filtered_firmware_configs = list(
            filter(
                lambda f: any(
                    fnmatch.fnmatch(full_machine_type, target_machine)
                    for target_machine in f["targets"][0]["machines"]
                ),
                filtered_firmware_configs,
            )
        )

        # Exclude Intel TDX and AMD SEV-ES firmwares.
        filtered_firmware_configs = list(
            filter(
                lambda f: "inteltdx" not in f["mapping"]["executable"]["filename"]
                and "amdsev" not in f["mapping"]["executable"]["filename"]
                # qcow2 does azl2, need to exclude such entries
                and "qcow2" not in f["mapping"]["executable"]["filename"],
                filtered_firmware_configs,
            )
        )

        # Filter on secure boot.
        if enable_secure_boot:
            filtered_firmware_configs = list(
                filter(
                    lambda f: "secure-boot" in f["features"]
                    and "enrolled-keys" in f["features"],
                    filtered_firmware_configs,
                )
            )
        else:
            filtered_firmware_configs = list(
                filter(
                    lambda f: "secure-boot" not in f["features"],
                    filtered_firmware_configs,
                )
            )

        # Get first matching firmware.
        firmware_config = next(iter(filtered_firmware_configs), None)
        if firmware_config is None:
            raise LisaException(
                f"Could not find matching firmware for machine-type={machine_type} "
                f"({full_machine_type}) and secure-boot={enable_secure_boot}."
            )

        return firmware_config

# Create XML definition for a VM.
def create_libvirt_domain_xml(vm_spec: VmSpec, host_os: str, boot_type: str, uefi_firmware_binary: str) -> str:
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
        loader.attrib["type"] = "pflash"
        loader.text = uefi_firmware_binary

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

    # We cannot use python's 'match' because Azure Linux 2.0 has an older
    # version of python that does not support it.
    # This code has been tested on Azure Linux 2.0 with Python 3.9.19.
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
