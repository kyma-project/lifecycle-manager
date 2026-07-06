#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

mkdir -p ~/registry-auth
docker run --rm httpd:2.4-alpine htpasswd -Bbn myuser mypass > ~/registry-auth/htpasswd

docker rm -f private-oci-reg.localhost 2>/dev/null || true
docker run -d \
  --restart=always \
  -p 5001:5000 \
  --name private-oci-reg.localhost \
  -v ~/registry-auth:/auth \
  -e "REGISTRY_AUTH=htpasswd" \
  -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
  -e "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd" \
  registry:2

docker network connect k3d-kcp private-oci-reg.localhost 2>/dev/null || true

kubectl create secret docker-registry private-oci-reg-creds \
  --docker-server=http://private-oci-reg.localhost:5000 \
  --docker-username=myuser \
  --docker-password=mypass \
  --docker-email=dummy@example.com \
  -n kcp-system \
  --dry-run=client -o yaml | kubectl apply -f -
