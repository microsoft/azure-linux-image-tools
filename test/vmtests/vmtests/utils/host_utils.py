# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.


def get_host_distro() -> str:
    file_path = "/etc/os-release"
    id_value = ""
    with open(file_path, "r") as file:
        for line in file:
            if line.startswith("ID="):
                id_value = line.strip().split("=", 1)[1]
                break

    if id_value == "":
        raise Exception("ID field not found in os-release file")

    return id_value
