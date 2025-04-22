{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "statuslistservice.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "statuslistservice.labels" -}}
helm.sh/chart: {{ include "statuslistservice.chart" . }}
{{ include "statuslistservice.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "statuslistservice.selectorLabels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
