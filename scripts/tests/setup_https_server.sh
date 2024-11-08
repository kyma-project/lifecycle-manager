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

[ alt_names ]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF


echo "Generating self-signed certificate..."
openssl req -x509 -newkey rsa:2048 -keyout $KEY_FILE -out $CERT_FILE -days 30 -nodes -config openssl.cnf

export SSL_CERT_FILE=$CERT_FILE
#echo "" | sudo -S cp $CERT_FILE /etc/ssl/certs/
#echo "" | sudo -S update-ca-certificates

# Start Python HTTPS server
echo "Serving $DIRECTORY_TO_SERVE on https://localhost:$PORT"
nohup python3 scripts/tests/https_server.py "$DIRECTORY_TO_SERVE" "$CERT_FILE" "$KEY_FILE" "$PORT" &
