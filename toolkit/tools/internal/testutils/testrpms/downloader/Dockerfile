ARG baseimage
FROM ${baseimage}
ARG imagecreator

RUN tdnf update -y && \
   tdnf install -y dnf dnf-plugins-core createrepo_c

RUN dnf download -y --resolve --alldeps --destdir /downloadedrpms \
   jq golang 
   
RUN if [ "${imagecreator}" = "true" ]; then \
    dnf download -y --resolve --alldeps --destdir /downloadedrpms \
      azurelinux-release azurelinux-repos azurelinux-rpm-macros bash ca-certificates ca-certificates-base \
      dbus dracut-hostonly e2fsprogs filesystem grub2 grub2-efi-binary iana-etc initramfs iproute iputils irqbalance \
      ncurses-libs openssl rpm rpm-libs shadow-utils shim sudo systemd systemd-networkd systemd-resolved \
      systemd-udev tdnf tdnf-plugin-repogpgcheck util-linux zlib kernel ; \
      fi

# Add repo metadata, so that the directory can be used in a .repo file.
RUN createrepo --compatibility --update /downloadedrpms
