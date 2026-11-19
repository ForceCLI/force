# Third Party Software License Attributions
{{ range . }}{{ if and (ne .LicensePath "Unknown") (ne .LicenseName "Unknown") }}
================================================================================
{{ .Name }}{{ if .Version }} {{ .Version }}{{ end }}
{{ .LicenseName }}
================================================================================

{{ .LicenseText }}
{{ end }}{{ end }}
