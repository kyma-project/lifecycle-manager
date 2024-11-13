#!/bin/bash

# Variables
DIRECTORY_TO_SERVE=$1

PORT=8080
CERT_FILE="server.crt"
KEY_FILE="server.key"

if [[ -z "$DIRECTORY_TO_SERVE" ]]; then
  echo "no directory provided to be served"
  exit 1
fi

cat <<EOF > openssl.cnf
[ req ]
default_bits       = 2048
default_keyfile    = privkey.pem
distinguished_name = req_distinguished_name
x509_extensions    = v3_ca
prompt              = no
encrypt_key         = no

[ req_distinguished_name ]
C  = US
ST = California
L  = San Francisco
O  = MyOrg
OU = Dev
CN = localhost  # Common Name, change to your domain/IP if needed

[ v3_ca ]
subjectAltName = @alt_names
keyUsage = nonRepudiation, digitalSignature, keyEncipherment

[ alt_names ]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF

echo "Generating certificate..."
openssl req -x509 -newkey rsa:2048 -keyout $KEY_FILE -out $CERT_FILE -days 30 -nodes -config openssl.cnf

# Create a local CA certificate store
mkdir -p $HOME/.local/share/ca-certificates

# Copy self-signed certificate to the store
cp $CERT_FILE $HOME/.local/share/ca-certificates/

# Update the certificate bundle
for cert in $HOME/.local/share/ca-certificates/*.crt; do
    ln -s "$cert" $HOME/.local/share/ca-certificates/$(openssl x509 -hash -noout -in "$cert").0
done

# Set the environment variable so that SSL/TLS libraries use the custom CA store
echo SSL_CERT_DIR=$HOME/.local/share/ca-certificates >> $GITHUB_ENV
echo CURL_CA_BUNDLE=$SSL_CERT_DIR/server.crt >> $GITHUB_ENV

# Start Python HTTPS server
echo "Serving $DIRECTORY_TO_SERVE on https://localhost:$PORT"
go run scripts/tests/https_server.go -dir "$DIRECTORY_TO_SERVE" -certfile "$CERT_FILE" -keyfile "$KEY_FILE" -port "$PORT" &

