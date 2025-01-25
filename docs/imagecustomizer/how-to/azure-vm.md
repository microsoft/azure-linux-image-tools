---
title: Create Azure VM
parent: How To
---

# Create a customized image and deploy it as an Azure VM

This guide shows you how to customize a marketplace Azure Linux image to include a
simple HTTP application written in Python and then deploy it to Azure as a VM.

## Words of caution

This is intended as an example only.

In general, for web applications, it is better to use a dedicated hosting service like
[Azure App Service](https://learn.microsoft.com/en-us/azure/app-service/overview),
[Azure Container Apps](https://learn.microsoft.com/en-us/azure/container-apps/), or
[Azure Kubernetes Service](https://learn.microsoft.com/en-us/azure/aks/what-is-aks)
than managing the VMs directly.

In addition, it is a good idea to stick both a load balancer (e.g.
[Azure Application Gateway](https://learn.microsoft.com/en-us/azure/application-gateway/overview))
and a CDN (Content Delivery Network) (e.g.
[Azure Front Door](https://learn.microsoft.com/en-us/azure/frontdoor/front-door-overview))
in front of any HTTP endpoints.

## Steps

1. Create a directory to stage all the build artifacts:

   ```bash
   STAGE_DIR="<staging-directory>"
   mkdir -p "$STAGE_DIR"
   ```

2. Download Azure Linux VHD file:
   [Download Azure Linux Marketplace Image](./download-marketplace-image.md)

3. Move the downloaded VHD file to the staging directory.

   ```bash
   mv ./image.vhd "$STAGE_DIR"
   ```

4. Create a file named `$STAGE_DIR/image-config.yaml` with the following
   contents:

   ```yaml
   os:
     additionalFiles:
       # Create a basic HTTP app using Flask.
     - destination: /home/myapp/myapp.py
       content: |
         from flask import Flask

         app = Flask(__name__)

         @app.route("/")
         def hello_world():
            return "<p>Hello, World!</p>"

       # Create a systemd service so that the app runs on OS boot.
     - destination: /usr/local/lib/systemd/system/myapp.service
       content: |
         [Unit]
         Description=My App

         [Service]
         Type=exec
         ExecStart=/home/myapp/venv/bin/waitress-serve --listen 127.0.0.1:8080 myapp:app
         User=myapp
         WorkingDirectory=~

         [Install]
         WantedBy=multi-user.target

       # Use nginx to proxy port 80 to port 8080.
       # This allows the app to run without root permissions.
     - destination: /etc/nginx/nginx.conf
       content: |
         worker_processes 1;

         events {
           worker_connections 1024;
         }

         http {
           include mime.types;

           server {
             listen 80;
             location / {
               proxy_pass http://127.0.0.1:8080;
               proxy_http_version 1.1;
               proxy_set_header Upgrade $http_upgrade;
               proxy_set_header Connection keep-alive;
               proxy_set_header Host $host;
               proxy_cache_bypass $http_upgrade;
             }
           }
         }

     packages:
       install:
       - nginx
       - python3
       - python3-pip

     services:
       enable:
       - nginx
       - myapp

     users:
     - name: myapp
       startupCommand: /usr/sbin/nologin

   scripts:
     postCustomization:
       # Install the required Python packages for the app.
     - content: |
         set -eux
         python3 -m venv /home/myapp/venv
         /home/myapp/venv/bin/pip3 install Flask waitress
   ```

5. Run Image Customizer to create the new image:

   ```bash
   IMG_CUSTOMIZER_TAG="mcr.microsoft.com/azurelinux/imagecustomizer:0.10.0"
   docker run \
     --rm \
     --privileged=true \
     -v /dev:/dev \
     -v "$STAGE_DIR:/mnt/staging:z" \
     "$IMG_CUSTOMIZER_TAG" \
     imagecustomizer \
       --image-file "/mnt/staging/image.vhd" \
       --config-file "/mnt/staging/image-config.yaml" \
       --build-dir "/mnt/staging/build" \
       --output-image-format "vhd-fixed" \
       --output-image-file "/mnt/staging/out/image.vhd" \
       --log-level debug
   ```

6. Create disk in Azure.

   ```bash
   LOCAL_DISK_PATH="$STAGE_DIR/out/image.vhd"
   DISK_SIZE="$(stat -c "$LOCAL_DISK_PATH")"

   DISK_NAME="<disk-name>"
   DISK_RG="<disk-resource-group-name>"
   DISK_LOC="<azure-location>"

   az group create --location "$DISK_LOC" --name "$DISK_RG"
   az disk create -n "$DISK_NAME" -g "$DISK_RG" --sku standard_lrs --os-type Linux \
     --hyper-v-generation 2 --for-upload --upload-size-bytes "$DISK_SIZE"
   ```

7. Upload new VHD to Azure.



## Helpful links

- [Upload a VHD to Azure - Azure CLI](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/disks-upload-vhd-to-managed-disk-cli)
