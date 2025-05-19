set -eux

notation cert add --type ca --store microsoft-supplychain-2022 "./Microsoft Supply Chain RSA Root CA 2022.crt"
notation policy import --force ./trustpolicy.json
