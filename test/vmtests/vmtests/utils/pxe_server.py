# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import os
import subprocess
import sys
import xml.etree.ElementTree as ET  # noqa: N817
from pathlib import Path
from typing import IO, Optional

import libvirt  # type: ignore

# Fixed addressing for the dedicated PXE network. These must not collide with libvirt's default network
# (192.168.122.0/24).
PXE_NETWORK_GATEWAY_IP = "192.168.123.1"
PXE_NETWORK_NETMASK = "255.255.255.0"
PXE_NETWORK_DHCP_START = "192.168.123.2"
PXE_NETWORK_DHCP_END = "192.168.123.254"
PXE_HTTP_PORT = 8080


def _build_pxe_network_xml(network_name: str, bridge_name: str, tftp_root: str, boot_loader_file: str) -> str:
    network = ET.Element("network")

    name = ET.SubElement(network, "name")
    name.text = network_name

    forward = ET.SubElement(network, "forward")
    forward.attrib["mode"] = "nat"

    bridge = ET.SubElement(network, "bridge")
    bridge.attrib["name"] = bridge_name
    bridge.attrib["stp"] = "on"
    bridge.attrib["delay"] = "0"

    ip = ET.SubElement(network, "ip")
    ip.attrib["address"] = PXE_NETWORK_GATEWAY_IP
    ip.attrib["netmask"] = PXE_NETWORK_NETMASK

    tftp = ET.SubElement(ip, "tftp")
    tftp.attrib["root"] = tftp_root

    dhcp = ET.SubElement(ip, "dhcp")

    dhcp_range = ET.SubElement(dhcp, "range")
    dhcp_range.attrib["start"] = PXE_NETWORK_DHCP_START
    dhcp_range.attrib["end"] = PXE_NETWORK_DHCP_END

    bootp = ET.SubElement(dhcp, "bootp")
    bootp.attrib["file"] = boot_loader_file

    return ET.tostring(network, "unicode")


def _make_world_readable(root: Path) -> None:
    os.chmod(root, 0o755)
    for dir_path, dir_names, file_names in os.walk(root):
        for dir_name in dir_names:
            os.chmod(os.path.join(dir_path, dir_name), 0o755)
        for file_name in file_names:
            os.chmod(os.path.join(dir_path, file_name), 0o644)


def _make_ancestors_traversable(leaf: Path) -> None:
    # Add the world-execute bit (traverse only, not read/list) to each ancestor that lacks it, all the way up to the
    # filesystem root, so the artifacts stay reachable without widening read access to any directory's contents.
    current = leaf.parent
    while True:
        mode = current.stat().st_mode
        if not mode & 0o001:
            os.chmod(current, mode | 0o001)
        if current == current.parent:
            break
        current = current.parent


# Stands up the network-boot environment a PXE VM test needs:
#
#   - A dedicated, transient libvirt NAT network whose embedded dnsmasq serves DHCP + TFTP. The TFTP root points at the
#     extracted PXE artifacts directory.
#   - A plain HTTP server over the same artifacts directory. The bootstrap initramfs downloads the full-OS image from
#     here. (HTTP is used for that transfer because it is far faster than TFTP for a large file.)
#
# It is Closeable so can be added to a test's close list.
class PxeEnvironment:
    def __init__(
        self,
        libvirt_conn: libvirt.virConnect,
        network_name: str,
        bridge_name: str,
        artifacts_dir: Path,
        boot_loader_file: str,
        http_log_file_path: Path,
    ):
        self.network_name: str = network_name

        self._network: Optional[libvirt.virNetwork] = None
        self._http_process: Optional[subprocess.Popen[bytes]] = None
        self._http_log: Optional[IO[str]] = None

        # pytest runs as root (via sudo) and creates the intermediate temp directories with mkdtemp (mode 0700). The
        # workspace root itself may be group-private (e.g. 0750). Since dnsmasq (TFTP) and qemu run as their own users,
        # ensure the artifacts are readable by others and can traverse into the artifacts directory.
        _make_world_readable(artifacts_dir)
        _make_ancestors_traversable(artifacts_dir)

        network_xml = _build_pxe_network_xml(network_name, bridge_name, str(artifacts_dir), boot_loader_file)
        logging.debug(f"Creating PXE libvirt network:\n{network_xml}")

        try:
            self._network = libvirt_conn.networkCreateXML(network_xml)

            logging.debug(f"Starting PXE HTTP server on port {PXE_HTTP_PORT} serving ({artifacts_dir})")
            self._http_log = open(http_log_file_path, "w", encoding="utf-8")
            self._http_process = subprocess.Popen(
                [
                    sys.executable,
                    "-m",
                    "http.server",
                    str(PXE_HTTP_PORT),
                    "--directory",
                    str(artifacts_dir),
                    # Bind only to the PXE NAT network's gateway IP, never 0.0.0.0. The artifacts are served
                    # unauthenticated over plain HTTP and are world-readable, so the listener must not be reachable from
                    # the build host's physical LAN. Binding to the gateway IP keeps it on the libvirt bridge interface
                    # only, so the guest VM under test can still reach it while other physical machines on the LAN
                    # cannot. The bridge interface already holds this IP because the libvirt network was created above.
                    "--bind",
                    PXE_NETWORK_GATEWAY_IP,
                ],
                stdout=self._http_log,
                stderr=subprocess.STDOUT,
            )
        except BaseException:
            self.close()
            raise

    def close(self) -> None:
        if self._http_process is not None:
            logging.debug("Stopping PXE HTTP server")
            self._http_process.terminate()
            try:
                self._http_process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self._http_process.kill()

            self._http_process = None

        if self._http_log is not None:
            self._http_log.close()
            self._http_log = None

        if self._network is not None:
            logging.debug(f"Destroying PXE libvirt network: {self.network_name}")
            try:
                self._network.destroy()
            except libvirt.libvirtError as ex:
                logging.warning(f"PXE network destroy failed. {ex}")

            self._network = None
