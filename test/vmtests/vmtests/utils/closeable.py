# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from typing import Protocol


# Interface for classes that have a 'close' method.
class Closeable(Protocol):
    def close(self) -> None:
        pass
