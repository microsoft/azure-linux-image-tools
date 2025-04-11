# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
from pathlib import Path
import platform
import time
from typing import Any, Optional

import libvirt  # type: ignore

from .ssh_client import SshClient, SshClientException


# Assists with creating and destroying a libvirt VM.
class LibvirtVm:
    def __init__(self, vm_name: str, domain_xml: str, libvirt_conn: libvirt.virConnect):
        self.vm_name: str = vm_name
        self.domain: libvirt.virDomain = None

        self.domain = libvirt_conn.defineXML(domain_xml)

    def start(self) -> None:
        # Start the VM in the paused state.
        # This gives the console logger a chance to connect before the VM starts.
        self.domain.createWithFlags(libvirt.VIR_DOMAIN_START_PAUSED)

        # PLACEHOLDER
        # Attach the console logger
        # self.console_logger = LibvirtConsoleLogger()
        # self.console_logger.attach(domain, console_log_file_path)

        # Start the VM.
        self.domain.resume()

    # Wait for the VM to boot and then get the IP address.
    def get_vm_ip_address(self, timeout: float = 30) -> str:
        start_time = time.time()
        timeout_time = start_time + timeout

        while True:
            addr = self.try_get_vm_ip_address()
            if addr:
                total_wait_time = time.time() - start_time
                logging.debug(f"Wait for VM ({self.vm_name}) boot / request IP address: {total_wait_time:.0f}s")
                return addr

            if time.time() > timeout_time:
                raise Exception(f"No IP addresses found for '{self.vm_name}'. OS might have failed to boot.")

            time.sleep(1)

    # Try to get the IP address of the VM.
    def try_get_vm_ip_address(self) -> Optional[str]:
        assert self.domain

        # Acquire IP address from libvirt's DHCP server.
        interfaces = self.domain.interfaceAddresses(libvirt.VIR_DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
        if len(interfaces) < 1:
            return None

        interface_name = next(iter(interfaces))
        addrs = interfaces[interface_name]["addrs"]
        if len(addrs) < 1:
            return None

        # For arm64 virtual machines, two IP addresses are assigned
        # temporarily - where the first is not usable and disappears after some
        # time. So, optimistically, we always return the last IP addrress in
        # the list assuming that is the one that works for all architectures.
        addr = addrs[len(addrs) - 1]["addr"]

        assert isinstance(addr, str)
        return addr

    def close(self) -> None:
        # Stop the VM.
        logging.debug(f"Stop VM: {self.vm_name}")
        try:
            # In the libvirt API, "destroy" means "stop".
            self.domain.destroy()
        except libvirt.libvirtError as ex:
            logging.warning(f"VM stop failed. {ex}")

        # PLACEHOLDER
        # Wait for console log to close.
        # Note: libvirt can deadlock if you try to undefine the VM while the stream
        # is trying to close.
        # if self.console_logger:
        #    log.debug(f"Close VM console log: {vm_name}")
        #    self.console_logger.close()
        #    self.console_logger = None

        # Undefine the VM.
        logging.debug(f"Delete VM: {self.vm_name}")
        try:
            self.domain.undefineFlags(
                libvirt.VIR_DOMAIN_UNDEFINE_MANAGED_SAVE
                | libvirt.VIR_DOMAIN_UNDEFINE_SNAPSHOTS_METADATA
                | libvirt.VIR_DOMAIN_UNDEFINE_NVRAM
                | libvirt.VIR_DOMAIN_UNDEFINE_CHECKPOINTS_METADATA
            )
        except libvirt.libvirtError as ex:
            logging.warning(f"VM delete failed. {ex}")

    def create_ssh_client(
        self,
        ssh_private_key_path: Path,
        test_temp_dir: Path,
    ) -> SshClient:

        ssh_known_hosts_path = test_temp_dir.joinpath("known_hosts")
        open(ssh_known_hosts_path, "w").close()

        # arm64 emulated runs take a very long time to boot and get to a state
        # where we can connect to it.
        ip_wait_time = 30
        if platform.machine() == 'aarch64':
            ip_wait_time = 300

        # For arm64 runs, we are seeing a behavior where the first IP address that
        # gets assigned becomes unusable by the time we try to ssh into the machine
        # and then ssh fails to connect.
        # Some time later, a different IP address gets assigned, and that IP
        # address is usable.
        stable_ip_time_out = 360
        stable_ip_wait_time = 120
        stable_ip_start_time = time.monotonic()
        while True:
            # Wait for VM to boot by waiting for it to request an IP address from the DHCP server.
            vm_ip_address = self.get_vm_ip_address(timeout=ip_wait_time)
            logging.debug(f"found IP address = {vm_ip_address}")

            # Connect to VM using SSH.
            try:
                vm_ssh = SshClient(vm_ip_address, key_path=ssh_private_key_path, known_hosts_path=ssh_known_hosts_path)
                return vm_ssh
            except SshClientException as e:
                delta_time = time.monotonic() - stable_ip_start_time
                if delta_time > stable_ip_time_out:
                    raise Exception(f"Error connecting to {vm_ip_address} - giving up: {e}")
                logging.debug(f"will retry the ssh connection in case the assigned IP address has changed")
                time.sleep(stable_ip_wait_time)

    def __enter__(self) -> "LibvirtVm":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()
