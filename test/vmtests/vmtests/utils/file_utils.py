# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path
from typing import Any


# Can be used with a `with` statement for deleting a file.
class RemoveFileOnClose:
    def __init__(self, path: Path):
        self.path: Path = path

    def close(self) -> None:
        self.path.unlink()

    def __enter__(self) -> "RemoveFileOnClose":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()
