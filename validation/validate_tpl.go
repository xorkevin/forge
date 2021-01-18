package validation

const templateValidate = `
func (r {{.Ident}}) {{.Prefix}}() error {
	{{- $prevalid := .PrefixValid -}}
	{{- $prehas := .PrefixHas -}}
	{{- $preopt := .PrefixOpt -}}
	{{- range .Fields -}}
	{{- $fnp := $prevalid -}}
	{{- if .Has -}}
		{{- $fnp = $prehas -}}
	{{- end }}
	{{- if .Opt -}}
		{{- $fnp = $preopt -}}
	{{- end }}
	if err := {{$fnp}}{{.Key}}(r.{{.Ident}}); err != nil {
		return err
	}
	{{- end }}
	return nil
}
`
