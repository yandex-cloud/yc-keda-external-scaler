{{- define "yc-keda-external-scaler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "yc-keda-external-scaler.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := include "yc-keda-external-scaler.name" . }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "yc-keda-external-scaler.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{ include "yc-keda-external-scaler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "yc-keda-external-scaler.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}
{{- end }}

{{- define "yc-keda-external-scaler.authMethod" -}}
{{- if .Values.auth.workloadIdentityFederation.serviceAccountID -}}
workloadIdentityFederation
{{- else -}}
authorizedKey
{{- end -}}
{{- end }}

{{- define "yc-keda-external-scaler.secretName" -}}
{{- if .Values.secret.existingSecret -}}
{{- .Values.secret.existingSecret -}}
{{- else if .Values.secret.create -}}
{{- default (include "yc-keda-external-scaler.fullname" .) .Values.secret.name -}}
{{- else -}}
{{- required "secret.existingSecret is required when secret.create=false (deprecated secretName is also accepted)" .Values.secretName -}}
{{- end -}}
{{- end }}

{{- define "yc-keda-external-scaler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yc-keda-external-scaler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "yc-keda-external-scaler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "yc-keda-external-scaler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
