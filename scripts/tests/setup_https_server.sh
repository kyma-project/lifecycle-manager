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

echo "Generating self-signed certificate..."
openssl req -x509 -newkey rsa:2048 -keyout $KEY_FILE -out $CERT_FILE -days 365 -nodes -subj "/CN=localhost"

# Start Python HTTPS server
echo "Serving $DIRECTORY_TO_SERVE on https://localhost:$PORT"
python3 scripts/tests/https_server.py "$DIRECTORY_TO_SERVE" "$CERT_FILE" "$KEY_FILE" "$PORT"
