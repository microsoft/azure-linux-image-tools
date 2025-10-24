set -eux

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
CERT_DIR="$SCRIPT_DIR/../../../internal/resources/certificates"

notation cert add --type ca --store microsoft-supplychain-2022 "$CERT_DIR/Microsoft Supply Chain RSA Root CA 2022.crt"
notation policy import --force "$SCRIPT_DIR/trustpolicy.json"
