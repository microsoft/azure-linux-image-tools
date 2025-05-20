set -eux

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"

notation cert add --type ca --store microsoft-supplychain-2022 "$SCRIPT_DIR/Microsoft Supply Chain RSA Root CA 2022.crt"
notation policy import --force "$SCRIPT_DIR/trustpolicy.json"
