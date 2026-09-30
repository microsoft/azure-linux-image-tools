# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import hashlib
from pathlib import Path
from typing import List, Optional
from unittest.mock import MagicMock, patch

import pytest

from ..utils.closeable import Closeable
from . import test_pxe


@pytest.mark.parametrize("fail_at", [None, 1, 2])
def test_pxe_diagnostic_boots_stop_on_failure(tmp_path: Path, fail_at: Optional[int]) -> None:
    config = tmp_path / "config.yaml"
    config.write_text("os: {}\n")
    image = tmp_path / "pxe-artifacts.tar.gz"
    image.write_bytes(b"unchanged PXE image")
    logs = tmp_path / "logs"
    close_list: List[Closeable] = []
    guests = [MagicMock(), MagicMock(), MagicMock()]
    for guest in guests:
        guest.domain.XMLDesc.return_value = "<domain/>"

    factory = MagicMock(side_effect=guests)
    customize = MagicMock()
    checks = MagicMock()
    if fail_at is not None:
        checks.side_effect = [None] * (fail_at - 1) + [RuntimeError("guest froze")]

    environment = MagicMock()
    environment.network_name = "diagnostic-network"
    with patch.multiple(
        test_pxe,
        add_ssh_to_config=MagicMock(return_value=config),
        add_pxe_bootstrap_base_url_to_config=MagicMock(return_value=config),
        get_username=MagicMock(return_value="testuser"),
        get_host_distro=MagicMock(return_value="ubuntu"),
        run_image_customizer=customize,
        create_libvirt_domain_xml=MagicMock(return_value="<domain/>"),
        PxeEnvironment=MagicMock(return_value=environment),
        LibvirtVm=factory,
        run_basic_checks=checks,
    ):
        args = (
            MagicMock(),
            "imagecustomizer:diagnostic",
            tmp_path / "base.vhdx",
            3,
            "bootstrap",
            config,
            ("public-key", tmp_path / "private-key"),
            tmp_path,
            "pxe-repro",
            logs,
            MagicMock(),
            close_list,
        )
        if fail_at is None:
            test_pxe.run_pxe_test(*args, boot_count=3)
        else:
            with pytest.raises(RuntimeError, match="guest froze"):
                test_pxe.run_pxe_test(*args, boot_count=3)

    attempted = fail_at or 3
    customize.assert_called_once()
    assert factory.call_count == attempted
    assert checks.call_count == attempted
    assert close_list == [environment, guests[attempted - 1]]
    for guest in guests[: attempted - 1]:
        guest.close.assert_called_once()
    guests[attempted - 1].close.assert_not_called()

    evidence = logs / "pxe-repro"
    assert (evidence / "image.sha256").read_text().split()[0] == hashlib.sha256(image.read_bytes()).hexdigest()
    assert (evidence / "config.yaml").read_bytes() == config.read_bytes()
    assert len(list(evidence.glob("boot*/domain.active.xml"))) == attempted
    assert len({call.args[2] for call in factory.call_args_list}) == attempted
    if fail_at is None:
        assert not (evidence / "pxe-artifacts.tar.gz").exists()
    else:
        assert (evidence / "pxe-artifacts.tar.gz").read_bytes() == image.read_bytes()
