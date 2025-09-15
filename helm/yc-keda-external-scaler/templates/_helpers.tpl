{{- define "yc-keda-external-scaler.name" -}}
yc-keda-external-scaler
{{- end }}

{{- define "yc-keda-external-scaler.fullname" -}}
{{ include "yc-keda-external-scaler.name" . }}
{{- end }}

{{- define "yc-keda-external-scaler.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{ include "yc-keda-external-scaler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
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
