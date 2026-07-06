# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import multiprocessing
import os
import shutil
import sys
import tarfile
import tempfile
import xml.etree.ElementTree as ET  # noqa: N817
from functools import partial
from http.server import HTTPServer, SimpleHTTPRequestHandler
from pathlib import Path
from typing import Optional

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


def _serve_http(directory: Path, bind_ip: str, port: int, log_file_path: Path) -> None:
    with open(log_file_path, "w", encoding="utf-8", buffering=1) as log_file:
        sys.stdout = log_file
        sys.stderr = log_file
        handler = partial(SimpleHTTPRequestHandler, directory=str(directory))
        with HTTPServer((bind_ip, port), handler) as httpd:
            httpd.serve_forever()


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
        pxe_tar_path: Path,
        boot_loader_file: str,
        http_log_file_path: Path,
    ):
        self.network_name: str = network_name

        self._network: Optional[libvirt.virNetwork] = None
        self._http_process: Optional[multiprocessing.Process] = None
        self._artifacts_dir: Optional[Path] = None

        try:
            # Extract the artifacts under /var/tmp. dnsmasq (TFTP) and qemu run as their own unprivileged users, so
            # every directory from the filesystem root down to the artifacts must be traversable by others. /var/tmp is
            # world-traversable (1777) and on disk, unlike the tmpfs-backed /tmp (the artifacts include the full-OS
            # image, which is too large for RAM).
            artifacts_dir = Path(tempfile.mkdtemp(prefix="pxe-artifacts-", dir="/var/tmp"))
            self._artifacts_dir = artifacts_dir
            with tarfile.open(pxe_tar_path, "r:gz") as tar:
                tar.extractall(artifacts_dir)

            # mkdtemp created the directory 0700 and the tarball preserves its own modes, so widen the extracted tree
            # to be readable and traversable by others.
            _make_world_readable(artifacts_dir)

            network_xml = _build_pxe_network_xml(network_name, bridge_name, str(artifacts_dir), boot_loader_file)
            logging.debug(f"Creating PXE libvirt network:\n{network_xml}")
            self._network = libvirt_conn.networkCreateXML(network_xml)

            logging.debug(f"Starting PXE HTTP server on port {PXE_HTTP_PORT} serving ({artifacts_dir})")
            # Bind only to the PXE NAT network's gateway IP, never 0.0.0.0. The artifacts are served
            # unauthenticated over plain HTTP and are world-readable, so the listener must not be reachable from
            # the build host's physical LAN. Binding to the gateway IP keeps it on the libvirt bridge interface
            # only, so the guest VM under test can still reach it while other physical machines on the LAN
            # cannot. The bridge interface already holds this IP because the libvirt network was created above.
            self._http_process = multiprocessing.Process(
                target=_serve_http,
                args=(artifacts_dir, PXE_NETWORK_GATEWAY_IP, PXE_HTTP_PORT, http_log_file_path),
                daemon=True,
            )
            self._http_process.start()
        except BaseException:
            self.close()
            raise

    def close(self) -> None:
        if self._http_process is not None:
            logging.debug("Stopping PXE HTTP server")
            self._http_process.terminate()
            self._http_process.join(timeout=10)
            if self._http_process.is_alive():
                self._http_process.kill()
                self._http_process.join()

            self._http_process = None

        if self._network is not None:
            logging.debug(f"Destroying PXE libvirt network: {self.network_name}")
            try:
                self._network.destroy()
            except libvirt.libvirtError as ex:
                logging.warning(f"PXE network destroy failed. {ex}")

            self._network = None

        if self._artifacts_dir is not None:
            logging.debug(f"Removing PXE artifacts directory: {self._artifacts_dir}")
            shutil.rmtree(self._artifacts_dir, ignore_errors=True)
            self._artifacts_dir = None
