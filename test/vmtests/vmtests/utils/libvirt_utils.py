# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import fnmatch
import json
import logging
import xml.etree.ElementTree as ET  # noqa: N817
import libvirt
import os
from pathlib import Path
import platform
from typing import Dict, Any


class VmSpec:
    def __init__(self, name: str, memory_mib: int, core_count: int, os_disk_path: Path, boot_type: str, secure_boot: bool):
        self.name: str = name
        self.memory_mib: int = memory_mib
        self.core_count: int = core_count
        self.os_disk_path: Path = os_disk_path
        self.boot_type: str = boot_type
        self.secure_boot: bool = secure_boot


def _get_libvirt_firmware_config(
        libvirt_conn: libvirt.virConnect,
        secure_boot: bool,
        machine_model: str,
        virt_type: str,
    ) -> Dict[str, Any]:
        # Resolve the machine type to its full name.
        domain_caps_str = libvirt_conn.getDomainCapabilities(machine=machine_model, virttype=virt_type)
        domain_caps = ET.fromstring(domain_caps_str)

        full_machine_type = domain_caps.findall("./machine")[0].text
        arch = domain_caps.findall("./arch")[0].text

        # Read the QEMU firmware config files, and build a list of json objects
        # Note: "/usr/share/qemu/firmware" is a well known location for these files.
        # Loop through all .json files in the folder
        decoder = json.JSONDecoder()
        firmware_configs = []
        for firmware_definition_file in Path("/usr/share/qemu/firmware").glob("*.json"):
            try:
                with firmware_definition_file.open("r", encoding="utf-8") as f:
                    data = f.read().lstrip()  # decode hates leading whitespace
                    while data:
                        obj, index = decoder.raw_decode(data)
                        firmware_configs.append(obj)
                        data = data[index:].lstrip()
            except json.JSONDecodeError as e:
                raise Exception(f"Error reading {firmware_definition_file.name}: {e}")

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
                lambda f: "executable" in f["mapping"]
                and "inteltdx" not in f["mapping"]["executable"]["filename"]
                and "amdsev" not in f["mapping"]["executable"]["filename"]
                # qcow2 does azl2, need to exclude such entries
                and "qcow2" not in f["mapping"]["executable"]["filename"],
                filtered_firmware_configs,
            )
        )

        # Filter on secure boot.
        if secure_boot:
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
            raise Exception(
                f"Could not find matching firmware for machine-type={machine_type} "
                f"({full_machine_type}) and secure-boot={secure_boot}."
            )

        return firmware_config

# Create XML definition for a VM.
def create_libvirt_domain_xml(libvirt_conn: libvirt.virConnect, vm_spec: VmSpec, log_file: str) -> str:

    host_arch = platform.machine()

    secure_boot_str = "yes" if vm_spec.secure_boot else "no"

    if host_arch == "x86_64":
        domain_type = "kvm"
        machine_model = "q35"
        virt_type="kvm"
        serial_target_type = "isa-serial"
        serial_target_model_name = "isa-serial"
    else:
        domain_type = "qemu"
        machine_model = "virt-6.2"
        virt_type = "qemu"
        serial_target_type = "system-serial"
        serial_target_model_name = "pl011"

    firmware_config = _get_libvirt_firmware_config(libvirt_conn, vm_spec.secure_boot, machine_model, virt_type)
    firmware_file = firmware_config["mapping"]["executable"]["filename"]

    domain = ET.Element("domain")
    domain.attrib["type"] = domain_type

    name = ET.SubElement(domain, "name")
    name.text = vm_spec.name

    memory = ET.SubElement(domain, "memory")
    memory.attrib["unit"] = "MiB"
    memory.text = str(vm_spec.memory_mib)

    vcpu = ET.SubElement(domain, "vcpu")
    vcpu.text = str(vm_spec.core_count)

    os_tag = ET.SubElement(domain, "os")

    os_type = ET.SubElement(os_tag, "type")
    os_type.text = "hvm"
    os_type.attrib["arch"] = host_arch
    os_type.attrib["machine"] = machine_model

    nvram = ET.SubElement(os_tag, "nvram")

    os_boot = ET.SubElement(os_tag, "boot")

    if vm_spec.boot_type == "efi":
        loader = ET.SubElement(os_tag, "loader")
        loader.attrib["readonly"] = "yes"
        loader.attrib["secure"] = secure_boot_str
        loader.attrib["type"] = "pflash"
        loader.text = firmware_file

    features = ET.SubElement(domain, "features")

    ET.SubElement(features, "acpi")
    ET.SubElement(features, "apic")

    cpu = ET.SubElement(domain, "cpu")
    if host_arch == "x86_64":
        cpu.attrib["mode"] = "host-passthrough"
    else:
        cpu.attrib["mode"] = "custom"
        cpu.attrib["match"] = "exact"
        cpu.attrib["check"] = "none"
        cp_model = ET.SubElement(cpu, "model")
        cp_model.attrib["fallback"] = "forbid"
        cp_model.text = "cortex-a57"

    clock = ET.SubElement(domain, "clock")
    clock.attrib["offset"] = "utc"

    on_poweroff = ET.SubElement(domain, "on_poweroff")
    on_poweroff.text = "destroy"

    on_reboot = ET.SubElement(domain, "on_reboot")
    on_reboot.text = "restart"

    on_crash = ET.SubElement(domain, "on_crash")
    on_crash.text = "destroy"

    devices = ET.SubElement(domain, "devices")

    if host_arch == "aarch64":
        emulator = ET.SubElement(devices, "emulator")
        emulator.text = "/usr/bin/qemu-system-aarch64"

        controller_scsi = ET.SubElement(devices, "controller")
        controller_scsi.attrib["type"] = "scsi"
        controller_scsi.attrib["index"] = "0"
        controller_scsi.attrib["model"] = "virtio-scsi"

    serial = ET.SubElement(devices, "serial")
    serial.attrib["type"] = "file"
    serial_source = ET.SubElement(serial, "source")
    serial_source.attrib["path"] = log_file

    serial_target = ET.SubElement(serial, "target")
    serial_target.attrib["type"] = serial_target_type
    serial_target.attrib["port"] = "0"

    serial_target_model = ET.SubElement(serial_target, "model")
    serial_target_model.attrib["name"] = serial_target_model_name

    console = ET.SubElement(devices, "console")
    console.attrib["type"] = "file"

    console_source = ET.SubElement(console, "source")
    console_source.attrib["path"] = log_file

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
        os_boot.attrib["dev"] = "hd"
        _add_disk_xml(
            devices=devices,
            file_path=str(vm_spec.os_disk_path),
            device_type="disk",
            image_type="qcow2",
            bus_type="virtio",
            device_prefix="vd",
            read_only=False,
            next_disk_indexes=next_disk_indexes
        )
    else:
        os_boot.attrib["dev"] = "cdrom"
        if host_arch == "x86_64":
            bus_type="sata"
        else:
            bus_type="scsi"

        _add_disk_xml(
            devices=devices,
            file_path=str(vm_spec.os_disk_path),
            device_type="cdrom",
            image_type="raw",
            bus_type=bus_type,
            device_prefix="sd",
            read_only=True,
            next_disk_indexes=next_disk_indexes
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
    read_only: bool,
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

    if read_only:
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
