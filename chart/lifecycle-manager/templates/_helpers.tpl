{{/*
Expand the name of the chart.
*/}}
{{- define "lifecycle-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "lifecycle-manager.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.fullnameOverride) | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create a default fully qualified name for watcher.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "lifecycle-manager.watcher.fullname" -}}
{{- if .Values.watcher.fullnameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.watcher.fullnameOverride) | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Values.watcher.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- else }}
{{- "watcher" }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "lifecycle-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "lifecycle-manager.labels" -}}
helm.sh/chart: {{ include "lifecycle-manager.chart" . }}
{{ include "lifecycle-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if and (hasKey .Values "global") (hasKey .Values.global "landscape") .Values.global.landscape }}
app.kubernetes.io/part-of: {{ printf "kcp-%s" .Values.global.landscape }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "lifecycle-manager.selectorLabels" -}}
app.kubernetes.io/component: {{ printf "%s.kyma-project.io" .Chart.Name  }}
app.kubernetes.io/name: {{ include "lifecycle-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create unified annotations for lifecycle manager CRDs
*/}}
{{- define "lifecycle-manager.crds.annotations" -}}
{{- $Release :=(.helm).Release | default .Release -}}
helm.sh/resource-policy: keep
meta.helm.sh/release-namespace: {{ .Release.Namespace }}
meta.helm.sh/release-name: {{ $Release.Name }}
{{- end -}}

{{/* Service account name */}}
{{- define "lifecycle-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "lifecycle-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Cert Manager apiVersion */}}
{{- define "lifecycle-manager.certmanager.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
cert-manager.io/v1
{{- else -}}
cert.gardener.cloud/v1alpha1
{{- end -}}
{{- end -}}

{{/* Cert Manager subject DN */}}
{{- define "lifecycle-manager.certmanager.subjectDN" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
subject:
  organizationalUnits:
    - {{ .Values.manager.certificateSubjectDN.organizationalUnit }}
  organizations:
    - {{ .Values.manager.certificateSubjectDN.organization }}
  localities:
    - {{ .Values.manager.certificateSubjectDN.locality }}
  provinces:
    - {{ .Values.manager.certificateSubjectDN.province }}
  countries:
    - {{ .Values.manager.certificateSubjectDN.country }}
{{- else -}}
{{- end -}}
{{- end -}}

{{/* Watcher Issuer Namespace */}}
{{- define "lifecycle-manager.watcher.issuer.namespace" -}}
{{- if .Values.watcher.issuerNamespace }}
{{- .Values.watcher.issuerNamespace }}
{{- else -}}
istio-system
{{- end -}}
{{- end -}}

{{/* Webhook Issuer Namespace */}}
{{- define "lifecycle-manager.webhook.issuer.namespace" -}}
{{ if .Values.webhook.issuerNamespace }}
{{- .Values.webhook.issuerNamespace }}
{{- else -}}
{{ .Release.Namespace }}
{{- end -}}
{{- end -}}

{{/* Watcher root issuer */}}
{{- define "lifecycle-manager.watcher.rootIssuerName" -}}
{{- if .Values.watcher.rootIssuerNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.watcher.rootIssuerNameOverride) }}
{{- else }}
{{- .Values.watcher.rootIssuerNameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-root" (include "lifecycle-manager.watcher.fullname" .) }}
{{- end }}
{{- end }}

{{/* Watcher selfsigned issuer */}}
{{- define "lifecycle-manager.watcher.issuerName" -}}
{{- if .Values.watcher.issuerNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.watcher.issuerNameOverride) }}
{{- else }}
{{- .Values.watcher.issuerNameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-selfsigned" (include "lifecycle-manager.watcher.fullname" .) }}
{{- end }}
{{- end }}

{{/* Watcher issuerRef */}}
{{- define "lifecycle-manager.watcher.issuerRef" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
group: cert-manager.io
kind: Issuer
name: {{ include "lifecycle-manager.watcher.rootIssuerName" . }}
{{- else -}}
name: {{ include "lifecycle-manager.watcher.rootIssuerName" . }}
namespace: {{ include "lifecycle-manager.watcher.issuer.namespace" . }}
{{- end -}}
{{- end -}}

{{/* Watcher serving cert name */}}
{{- define "lifecycle-manager.watcher.certName" -}}
{{- if .Values.watcher.certNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.watcher.certNameOverride) }}
{{- else }}
{{- .Values.watcher.certNameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-serving" (include "lifecycle-manager.watcher.fullname" .) }}
{{- end }}
{{- end }}

{{/* Watcher secret name */}}
{{- define "lifecycle-manager.watcher.clusterSecretName" -}}
{{- if .Values.watcher.secretNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.watcher.secretNameOverride) }}
{{- else }}
{{- .Values.watcher.secretNameOverride}}
{{- end }}
{{- else }}
{{- include "lifecycle-manager.watcher.fullname" . }}
{{- end }}
{{- end }}

{{/* Cert Manager watcher clusterSecretRef */}}
{{- define "lifecycle-manager.watcher.clusterSecretRef" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
secretName: {{ include "lifecycle-manager.watcher.clusterSecretName" . }}
{{- else -}}
privateKeySecretRef:
  name: {{ include "lifecycle-manager.watcher.clusterSecretName" . }}
  namespace: istio-system
{{- end -}}
{{- end -}}

{{/* Cert Manager watcher CA cert private key config */}}
{{- define "lifecycle-manager.watcher.privateKey" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
{{ .Files.Get "files/cert-keys/klm-watcher-serving.yaml" }}
{{- else -}}
{{ .Files.Get "files/cert-keys/klm-watcher-serving-gardener.yaml" }}
{{- end -}}
{{- end -}}

{{/* Cert Manager watcher CA cert secret labels */}}
{{- define "lifecycle-manager.watcher.secretLabels" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
secretTemplate:
  labels:
    operator.kyma-project.io/managed-by: lifecycle-manager
{{- else -}}
secretLabels:
  operator.kyma-project.io/managed-by: lifecycle-manager
{{- end -}}
{{- end -}}

{{/* Cert Manager watcher rules */}}
{{- define "lifecycle-manager.watcher.rules" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
{{ .Files.Get "files/rbac-rules/klm-certmanager-rules.yaml" }}
{{- else -}}
{{ .Files.Get "files/rbac-rules/klm-gardener-cm-rules.yaml" }}
{{- end -}}
{{- end -}}

{{/* Controller manager webhook issuer */}}
{{- define "lifecycle-manager.webhook.issuerName" -}}
{{- if .Values.webhook.issuerNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.webhook.issuerNameOverride) }}
{{- else }}
{{- .Values.webhook.issuerNameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-selfsigned" (include "lifecycle-manager.fullname" .) }}
{{- end }}
{{- end }}

{{/* Cert Manager webhook issuerRef */}}
{{- define "lifecycle-manager.webhook.issuerRef" -}}
{{- if .Capabilities.APIVersions.Has "cert-manager.io/v1" -}}
kind: Issuer
name: {{ include "lifecycle-manager.webhook.issuerName" . }}
{{- else -}}
name: {{ include "lifecycle-manager.webhook.issuerName" . }}
namespace: {{ include "lifecycle-manager.webhook.issuer.namespace" . }}
{{- end -}}
{{- end -}}

{{/* Webhook serving cert name */}}
{{- define "lifecycle-manager.webhook.certName" -}}
{{- if .Values.webhook.certNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.webhook.certNameOverride) }}
{{- else }}
{{- .Values.webhook.certNameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-webhook-serving" (include "lifecycle-manager.fullname" .) }}
{{- end }}
{{- end }}

{{/* Webhook secret name */}}
{{- define "lifecycle-manager.webhook.secretName" -}}
{{- if .Values.webhook.secretNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.webhook.secretNameOverride) }}
{{- else }}
{{- .Values.webhook.secretNameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-webhook" (include "lifecycle-manager.fullname" .) }}
{{- end }}
{{- end }}

{{/* Controller manager base cluster role */}}
{{- define "lifecycle-manager.role.managerName" -}}
{{- (include "lifecycle-manager.fullname" .) }}
{{- end }}

{{/* Controller manager CRD cluster role */}}
{{- define "lifecycle-manager.role.managerCRDName" -}}
{{- printf "%s-crds" (include "lifecycle-manager.fullname" .) }}
{{- end }}

{{/* Controller manager leader election role */}}
{{- define "lifecycle-manager.role.leaderElectionName" -}}
{{- printf "%s-leader-election" (include "lifecycle-manager.fullname" .) }}
{{- end }}

{{/* Controller manager skr role */}}
{{- define "lifecycle-manager.role.skr" -}}
{{- printf "%s-skr" (include "lifecycle-manager.fullname" .) }}
{{- end }}

{{/* Controller manager certmanager role */}}
{{- define "lifecycle-manager.role.certmanager" -}}
{{- printf "%s-certmanager" (include "lifecycle-manager.fullname" .) }}
{{- end }}

{{- /*
TODO: this seems to be unused, can be removed?
*/}}
{{/* config name */}}
{{- define "lifecycle-manager.configName" -}}
{{- if .Values.prefix }}
{{- printf "%s-manager-config" .Values.prefix }}
{{- else }}
{{- printf "%s-manager-config" .Release.Name }}
{{- end }}
{{- end }}

{{/* Watcher gateway name */}}
{{- define "lifecycle-manager.watcher.gatewayName" -}}
{{- if .Values.watcher.gatewayNameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.watcher.gatewayNameOverride) }}
{{- else }}
{{- .Values.watcher.gatewayNameOverride}}
{{- end }}
{{- else }}
{{- include "lifecycle-manager.watcher.fullname" . }}
{{- end }}
{{- end }}

{{/* Controller manager event service name */}}
{{- define "lifecycle-manager.events.serviceName" -}}
{{- if .Values.service.listener.nameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.service.listener.nameOverride) }}
{{- else }}
{{- .Values.service.listener.nameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-events" (include "lifecycle-manager.fullname" .) }}
{{- end }}
{{- end }}

{{/* Controller manager metrics service name */}}
{{- define "lifecycle-manager.metrics.serviceName" -}}
{{- if .Values.service.metrics.nameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.service.metrics.nameOverride) }}
{{- else }}
{{- .Values.service.metrics.nameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-metrics" (include "lifecycle-manager.fullname" .) }}
{{- end }}
{{- end }}

{{/* Webhook service name */}}
{{- define "lifecycle-manager.webhook.serviceName" -}}
{{- if .Values.service.webhook.nameOverride }}
{{- if .Values.prefix }}
{{- (printf "%s-%s" .Values.prefix .Values.service.webhook.nameOverride) }}
{{- else }}
{{- .Values.service.webhook.nameOverride}}
{{- end }}
{{- else }}
{{- printf "%s-webhook" (include "lifecycle-manager.fullname" .) }}
{{- end }}
{{- end }}

{{/* vmscrape/servicemonitor specs */}}
{{- define "lifecycle-manager.monitorEndpoints" }}
- path: /metrics
  port: metrics
{{- if eq .type "vm" }}
  attach_metadata: {}
{{- end }}
{{- if not .ctx.Values.global.istio.ambient.enabled }}
  scheme: https
  tlsConfig:
    caFile: /etc/vm/secrets/istio.default/root-cert.pem
    certFile: /etc/vm/secrets/istio.default/cert-chain.pem
    insecureSkipVerify: true
    keyFile: /etc/vm/secrets/istio.default/key.pem
{{ else }}
  scheme: http
{{- end }}

{{- end }}

{{- define "istio.waypointLabels" }}
istio.io/ingress-use-waypoint: "true"
istio.io/use-waypoint: waypoint
{{- end }}

{{- define "istio.ambientLabels" }}
istio.io/dataplane-mode: ambient
sidecar.istio.io/inject: "false"
{{- end }}
