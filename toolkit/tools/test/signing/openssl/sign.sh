set -eux

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"

ARTIFACTS_DIR="$1"

NSSDB="$SCRIPT_DIR/nssdb/"

rm -rf "$NSSDB"
mkdir -p "$NSSDB"

rm -rf "$SCRIPT_DIR/tmp"
mkdir -p "$SCRIPT_DIR/tmp"

certutil -N -d "$NSSDB" --empty-password
certutil -A -n myca -d "$NSSDB" -i "$SCRIPT_DIR/myca.crt" -t ,,u

shopt -s globstar
for FILE in "$ARTIFACTS_DIR"/**/*.efi; do
    # Export digest metadata
    pesign --in "$FILE" --export-signed-attributes "$SCRIPT_DIR/tmp/sattrs.bin" --force

    # Create signature
    openssl dgst -sign "$SCRIPT_DIR/myca.pem" \
        -sha256 \
        -out "$SCRIPT_DIR/tmp/sattrs.sig" "$SCRIPT_DIR/tmp/sattrs.bin"

    # Attach signature to file
    pesign \
        --certificate myca \
        --import-raw-signature "$SCRIPT_DIR/tmp/sattrs.sig" --import-signed-attributes "$SCRIPT_DIR/tmp/sattrs.bin" \
        --in "$FILE" --out "$FILE.tmp" \
        --certdir "$NSSDB"

    mv -f "$FILE.tmp" "$FILE"
done

# sudo virt-fw-vars --in-place <VARS> --add-db $(uuid) <CERT>
