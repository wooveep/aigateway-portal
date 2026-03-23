{{- define "aigateway-portal.name" -}}
{{- default .Chart.Name (default .Values.nameOverride .Values.name) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "aigateway-portal.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "aigateway-portal.name" . -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "aigateway-portal.labels" -}}
helm.sh/chart: {{ include "aigateway-portal.chart" . }}
app.kubernetes.io/name: {{ include "aigateway-portal.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "aigateway-portal.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aigateway-portal.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "aigateway-portal.backend.fullname" -}}
{{- include "aigateway-portal.fullname" . -}}
{{- end -}}

{{- define "aigateway-portal.backend.serviceAccountName" -}}
{{- if .Values.backend.serviceAccount.create -}}
{{- if .Values.backend.serviceAccount.name -}}
{{- .Values.backend.serviceAccount.name -}}
{{- else -}}
{{- printf "%s-backend" (include "aigateway-portal.backend.fullname" .) -}}
{{- end -}}
{{- else -}}
{{- default "default" .Values.backend.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.frontend.fullname" -}}
{{- include "aigateway-portal.fullname" . -}}
{{- end -}}

{{- define "aigateway-portal.mysql.host" -}}
{{- if .Values.database.host -}}
{{- .Values.database.host -}}
{{- else if .Values.mysql.enabled -}}
{{- if .Values.mysql.service.name -}}
{{- .Values.mysql.service.name -}}
{{- else -}}
{{- printf "%s-aigateway-portal-mysql" .Release.Name -}}
{{- end -}}
{{- else -}}
{{- printf "%s-aigateway-core-mysql" .Release.Name -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.dbSecretName" -}}
{{- if .Values.mysql.enabled -}}
{{- if .Values.mysql.auth.existingSecret -}}
{{- .Values.mysql.auth.existingSecret -}}
{{- else -}}
{{- printf "%s-aigateway-portal-db" .Release.Name -}}
{{- end -}}
{{- else -}}
{{- if .Values.database.existingSecret -}}
{{- .Values.database.existingSecret -}}
{{- else -}}
{{- printf "%s-aigateway-core-db" .Release.Name -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.sessionSecretName" -}}
{{- if .Values.session.existingSecret -}}
{{- .Values.session.existingSecret -}}
{{- else -}}
{{- printf "%s-session" (include "aigateway-portal.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.image.repository" -}}
{{- if and .Values.image .Values.image.repository -}}
{{- .Values.image.repository -}}
{{- else -}}
{{- .Values.backend.image.repository -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.image.tag" -}}
{{- if and .Values.image .Values.image.tag -}}
{{- .Values.image.tag -}}
{{- else -}}
{{- .Values.backend.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.image.pullPolicy" -}}
{{- if and .Values.image .Values.image.pullPolicy -}}
{{- .Values.image.pullPolicy -}}
{{- else -}}
{{- .Values.backend.image.pullPolicy -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.service.type" -}}
{{- if and .Values.service .Values.service.type -}}
{{- .Values.service.type -}}
{{- else -}}
{{- .Values.frontend.service.type -}}
{{- end -}}
{{- end -}}

{{- define "aigateway-portal.service.port" -}}
{{- if and .Values.service .Values.service.port -}}
{{- .Values.service.port -}}
{{- else if and .Values.frontend .Values.frontend.service .Values.frontend.service.port -}}
{{- .Values.frontend.service.port -}}
{{- else -}}
{{- .Values.backend.service.port -}}
{{- end -}}
{{- end -}}
