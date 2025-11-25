ARG baseimage
FROM ${baseimage}
ARG package_list=""

# Install necessary tools for package downloading (Fedora uses dnf natively)
RUN dnf install -y dnf-plugins-core createrepo_c

# Download packages specified in package_list
RUN if [ -n "${package_list}" ]; then \
        echo "Packages to download: ${package_list}" && \
        dnf download -y --resolve --alldeps --destdir /downloadedrpms ${package_list}; \
    else \
        echo "No packages specified to download"; \
    fi

# Add repo metadata, so that the directory can be used in a .repo file.
RUN createrepo --compatibility --update /downloadedrpms
