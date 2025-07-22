# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from getpass import getuser


# Get the name of the current user.
# This makes it easier for the user to manually SSH into the VM when debugging.
def get_username() -> str:
    sudo_user = os.environ.get("SUDO_USER")
    if sudo_user is not None:
        # User is using sudo.
        # So, use their actual user name instead of "root".
        return sudo_user

    user = getuser()
    if user == "root":
        # The root user is typically disabled for SSH.
        # So, use a different name.
        return "test"

    return user
