#! /bin/bash

for file in *.json; do
  echo "apiVersion: v1
kind: ConfigMap
metadata:
  name: "${file%.*}"-dashboard
  namespace: kcp-system
  labels:
    grafana_dashboard: "1"
    app: monitoring-grafana
data:
  "${file%.*}": |-
    "$(cat "${file##*/}" | jq -cr)"" > "${file%.*}"-configmap.yaml
done