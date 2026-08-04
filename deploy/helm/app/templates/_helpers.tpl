{{- define "app.name" -}}
{{- .Values.serviceName -}}
{{- end -}}

{{- define "app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{/*
Common labels applied to every object.
*/}}
{{- define "app.labels" -}}
app.kubernetes.io/name: {{ .Values.serviceName }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: order-fulfillment
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ include "app.chart" . }}
{{- end -}}

{{/*
Selector labels: stable across upgrades, so never include version here.
*/}}
{{- define "app.selectorLabels" -}}
app.kubernetes.io/name: {{ .Values.serviceName }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Hardened pod security context, shared by workloads and the migrate hook. The
distroless nonroot images run as uid 65532.
*/}}
{{- define "app.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
fsGroup: 65532
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "app.containerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop:
    - ALL
{{- end -}}
