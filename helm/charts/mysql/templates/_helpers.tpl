{{- define "aigateway-portal-mysql.name" -}}
aigateway-portal-mysql
{{- end -}}

{{- define "aigateway-portal-mysql.fullname" -}}
{{- if .Values.service.name -}}
{{- .Values.service.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-aigateway-portal-mysql" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal-mysql.authSecretName" -}}
{{- if .Values.auth.existingSecret -}}
{{- .Values.auth.existingSecret -}}
{{- else -}}
{{- printf "%s-aigateway-portal-db" .Release.Name -}}
{{- end -}}
{{- end -}}
