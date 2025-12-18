---
title: Telemetry
parent: Image Customizer
nav_order: 7
---

# Telemetry in Image Customizer

Image Customizer collects usage data to help improve the product. This data
helps us understand usage patterns, diagnose issues, improve reliability and
prioritize new features based on real-world usage. This section describes what
telemetry is collected, how it’s used, and how to opt out. 

## Scope 

Image Customizer can be run in two ways:

- As a container image downloaded from Microsoft Artifact Registry
- As a binary built from the GitHub repository

Telemetry is collected in both cases.

## How to opt out 

Telemetry collection for Image Customizer is enabled by default. To opt out, use
the flag `--disable-telemetry`. Here is a sample command: 

```bash
docker run \
   --rm \
   --privileged=true \
   -v /dev:/dev \
   -v "$HOME/staging:/mnt/staging:z" \
   mcr.microsoft.com/azurelinux/imagecustomizer:0.18.0 \
     --image-file "/mnt/staging/image.vhdx" \
     --config-file "/mnt/staging/image-config.yaml" \
     --build-dir "/build" \
     --output-image-format "vhdx" \
     --output-image-file "/mnt/staging/out/image.vhdx"
     --disable-telemetry
```

### Data Collection and Storage 

Image Customizer doesn't collect personal data, such as IP addresses, usernames,
Azure subscription IDs or email addresses. Telemetry is collected using the
[OpenTelemetry (OTel)](https://learn.microsoft.com/en-us/azure/azure-monitor/app/opentelemetry-overview)
framework and is stored securely in an Azure Monitor Workspace to be used only
for product improvement and diagnostics. 

The telemetry feature collects the following data: 

| Category          |Examples                                                                  |
|-------------------|--------------------------------------------------------------------------|
| Usage Metrics     | Number of image customizations, API call frequency, Number of Image Customizer downloads |
| Environment Info  | Host OS (e.g. WSL, Ubuntu, Azure Linux), base image-format              |
| Performance       | Image creation success and failure rates, error codes, Time taken to create images |


Protecting your privacy is important to us. You can always opt out and continue
using the tool without restrictions.
