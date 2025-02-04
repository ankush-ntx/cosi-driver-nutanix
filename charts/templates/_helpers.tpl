{{/*
Expand the name of the chart.
*/}}
{{- define "cosi-driver-nutanix.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cosi-driver-nutanix.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "cosi-driver-nutanix.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cosi-driver-nutanix.labels" -}}
helm.sh/chart: {{ include "cosi-driver-nutanix.chart" . }}
{{ include "cosi-driver-nutanix.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cosi-driver-nutanix.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cosi-driver-nutanix.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cosi-driver-nutanix.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cosi-driver-nutanix.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create common labels for cosi related resources
*/}}
{{- define "cosi-driver-nutanix.resource.labels" -}}
app.kubernetes.io/component: controller
app.kubernetes.io/name: container-object-storage-interface-controller
app.kubernetes.io/part-of: container-object-storage-interface
app.kubernetes.io/version: main
{{- end }}

{{/*
Create common annotations for cosi related resources
*/}}
{{- define "cosi-driver-nutanix.resource.annotations" -}}
cosi.storage.k8s.io/authors: Nutanix Inc
cosi.storage.k8s.io/license: Apache V2
cosi.storage.k8s.io/support: https://github.com/kubernetes-sigs/container-object-storage-api
{{- end }}

{{/*
Create the full name of driver image from repository and tag
*/}}
{{- define "cosi-driver-nutanix.driverImageName" }}
  {{- .Values.provisioner.image.repository }}:{{ .Values.provisioner.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Create the full name of sidecar image from repository and tag
*/}}
{{- define "cosi-driver-nutanix.sidecarImageName" }}
  {{- .Values.objectstorageProvisionerSidecar.image.repository }}:{{ .Values.objectstorageProvisionerSidecar.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Create the full name of controller image from repository and tag
*/}}
{{- define "cosi-driver-nutanix.controllerImageName" }}
  {{- .Values.cosiController.image.repository }}:{{ .Values.cosiController.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Create the secret name
*/}}
{{- define "cosi-driver-nutanix.configSecretName" }}
  {{- if .Values.configuration.create }}
    {{- default (printf "%s-config" (include "cosi-driver-nutanix.name" . )) .Values.configuration.secretName }}
  {{- else }}
    {{- .Values.configuration.secretName }}
  {{- end }}
{{- end }}

{{/*
Create the name for secret volume
*/}}
{{- define "cosi-driver-nutanix.configVolumeName" }}
  {{- printf "%s-config" (include "cosi-driver-nutanix.name" . ) }}
{{- end }}
