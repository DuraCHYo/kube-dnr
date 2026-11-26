{{/*
Expand the name of the chart.
*/}}
{{- define "kube-dnr.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kube-dnr.fullname" -}}
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
{{- define "kube-dnr.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kube-dnr.labels" -}}
helm.sh/chart: {{ include "kube-dnr.chart" . }}
{{ include "kube-dnr.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kube-dnr.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-dnr.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Render the application config as pretty JSON without Go's HTML escaping.
Go's json.Marshal escapes <, > and & as \u003c, \u003e and \u0026, which breaks
external placeholder processors such as argocd-vault-plugin: they look for a
literal <path:...> and would never match the escaped form.
*/}}
{{- define "kube-dnr.configJson" -}}
{{- toPrettyJson . | replace "\\u003c" "<" | replace "\\u003e" ">" | replace "\\u0026" "&" -}}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kube-dnr.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kube-dnr.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
