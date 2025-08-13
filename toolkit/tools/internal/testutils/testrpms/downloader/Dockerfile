ARG baseimage
FROM ${baseimage}
ARG imagecreator

# Copy the configuration files for package list extraction
COPY fedora.yaml /tmp/fedora.yaml
COPY minimal-os.yaml /tmp/minimal-os.yaml

# Install necessary tools based on the base image
RUN if grep -q "Azure Linux" /etc/os-release 2>/dev/null || grep -q "CBL-Mariner" /etc/os-release 2>/dev/null; then \
        # Azure Linux - install dnf and createrepo_c via tdnf
        tdnf update -y && \
        tdnf install -y dnf dnf-plugins-core createrepo_c python3-pyyaml; \
    else \
        # Fedora - install createrepo_c and python3-yaml via dnf
        dnf update -y && \
        dnf install -y createrepo_c python3-pyyaml; \
    fi

# Download common packages and distribution-specific packages
RUN if grep -q "Azure Linux" /etc/os-release 2>/dev/null || grep -q "CBL-Mariner" /etc/os-release 2>/dev/null; then \
        # Azure Linux distribution - extract packages from minimal-os.yaml
        echo "Detected Azure Linux, extracting packages from minimal-os.yaml..." && \
        AZURELINUX_PACKAGES=$(python3 -c "import yaml; import sys; data=yaml.safe_load(open('/tmp/minimal-os.yaml')); print(' '.join(data['os']['packages']['install']))") && \
        echo "Azure Linux packages to download: $AZURELINUX_PACKAGES" && \
        dnf download -y --resolve --alldeps --destdir /downloadedrpms jq golang; \
        if [ "${imagecreator}" = "true" ]; then \
            dnf download -y --resolve --alldeps --destdir /downloadedrpms $AZURELINUX_PACKAGES; \
        fi; \
    else \
        # Fedora distribution - extract packages from fedora.yaml
        echo "Detected Fedora, extracting packages from fedora.yaml..." && \
        FEDORA_PACKAGES=$(python3 -c "import yaml; import sys; data=yaml.safe_load(open('/tmp/fedora.yaml')); print(' '.join(data['os']['packages']['install']))") && \
        echo "Fedora packages to download: $FEDORA_PACKAGES" && \
        dnf download -y --resolve --alldeps --destdir /downloadedrpms jq golang; \
        if [ "${imagecreator}" = "true" ]; then \
            dnf download -y --resolve --alldeps --destdir /downloadedrpms $FEDORA_PACKAGES; \
        fi; \
    fi

# Add repo metadata, so that the directory can be used in a .repo file.
RUN createrepo --compatibility --update /downloadedrpms
