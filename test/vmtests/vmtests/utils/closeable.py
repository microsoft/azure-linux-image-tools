from typing import Protocol


# Interface for classes that have a 'close' method.
class Closeable(Protocol):
    def close(self) -> None:
        pass
