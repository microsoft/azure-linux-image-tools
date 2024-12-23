# Image History

The Image History feature records the customization history of images in a
structured JSON format. It enables users to:

- Trace the lineage of customized images.
- Verify input integrity using SHA256 hashes.
- Debug and identify issues in customization workflows.


## History Storage

- **Location:** `/usr/share/image-customizer/history.json`.
- **Format:** An array of JSON objects, each representing a customization step.
- **Order:** Each new history entry is appended to the end of the existing list
  within the history file, preserving chronological sequence across image
  customizations.

## Metadata Captured

Each entry in the history includes:

- **Timestamp:** Time of customization.
- **Tool Version:** Image customizer version used.
- **Image UUID:** Unique identifier for the image.
- **Configuration:** Captures the config used for customization (with certain
  modifications).

## Config Modifications

The configuration undergoes specific modifications before being serialized into
JSON format:

**Hashing:**

- SHA256 hashes are generated for scripts, additional files, and the contents of
  additional directories to ensure input integrity.
- If only content is provided for scripts or additional files (and no source
  path), no hash is calculated or added. When a source path is specified, a
  SHA256 hash is computed and included as a field in the JSON configuration.
- For additional directories, a hashmap of each file to its SHA256 hash is added
  as a field in the JSON configuration.

**Redaction:** Known sensitive information, such as SSH public keys, is replaced
with `[redacted]` to maintain security.

## JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "ImageHistory",
  "type": "object",
  "properties": {
    "timestamp": {
      "type": "string",
      "format": "date-time",
      "description": "The timestamp for when the image was built."
    },
    "toolVersion": {
      "type": "string",
      "description": "The version of the tool used for the customization."
    },
    "imageUuid": {
      "type": "string",
      "description": "A unique identifier for the customized image."
    },
    "config": {
      "type": "object",
      "description": "Configuration data with added/redacted fields."
    }
  },
  "required": ["timestamp", "toolVersion", "imageUuid", "config"]
}
```

## Example JSON Entry

```json
{
  "timestamp": "2024-12-09T17:20:54Z",
  "toolVersion": "0.1.0",
  "imageUuid": "4a0cb56c-efa8-6636-528f-033477a7bb27",
  "config": {
    "additionalFiles": [
      {
        "destination": "/a.txt",
        "source": "files/a.txt",
        "sha256hash": "06577bd4a35a3fb866f891567b5a9ff67223c2f4422fb7629836d0cadb603ed3"
      }
    ],
    "additionalDirs": [
      {
        "source": "dirs/a",
        "destination": "/",
        "sha256hashmap": {
          "usr/local/bin/animals.sh": "13115588883d6f8960cf2e5f03ffcd73babcbb8da69789683715911487c8b1b6"
        }
      }
    ],
    "users": [
      {
        "name": "test",
        "sshPublicKeys": "[redacted]"
      }
    ]
  }
}
```
