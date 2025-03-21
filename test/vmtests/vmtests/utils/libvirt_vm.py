# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import time
from typing import Any, Optional

import libvirt  # type: ignore


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

    def __enter__(self) -> "LibvirtVm":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()
