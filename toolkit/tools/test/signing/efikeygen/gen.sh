set -eux

rm -rf ./nssdb
mkdir -p ./nssdb

certutil -N -d ./nssdb/ --empty-password

efikeygen --ca --self-sign --nickname "My CA" --common-name "CN=My CA" --dbdir=./nssdb
efikeygen --kernel --signer "My CA" --nickname "My Signer" --common-name "CN=My Signer" --dbdir=./nssdb

pk12util -d ./nssdb/ -o myca.pk12 -n "My CA" -W ""
pk12util -d ./nssdb/ -o mysigner.pk12 -n "My Signer" -W ""

openssl x509 -in myca.pk12 -text -noout
openssl x509 -in mysigner.pk12 -text -noout
