set -eux

rm -f myca.crt myca.pem

openssl req -x509 -newkey rsa:2048 -days 1 \
  -noenc -keyout myca.pem -out myca.crt \
  -subj "/CN=My CA" \
  -sha256 \
  -addext "basicConstraints=CA:FALSE" \
  -addext "extendedKeyUsage=codeSigning"

openssl x509 -in myca.crt -text -noout
