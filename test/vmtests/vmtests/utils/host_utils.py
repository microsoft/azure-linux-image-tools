# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.


def get_host_distro() -> str:
    file_path = "/etc/os-release"
    name_value = ""
    with open(file_path, "r") as file:
        for line in file:
            if line.startswith("ID="):
                name_value = line.strip().split("=", 1)[1]  # Get the value part
                break
    if name_value == "":
        raise Exception("ID field not found in os-release file")

    return name_value
