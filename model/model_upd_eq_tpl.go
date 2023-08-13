package model

const templateUpdEq = `
func (t *{{.Prefix}}ModelTable) Upd{{.ModelIdent}}{{.Name}}(ctx context.Context, d sqldb.Executor, m *{{.ModelIdent}}, {{.SQLCond.IdentParams}}) error {
	{{- if .SQLCond.ArrIdentArgs }}
	paramCount := {{.SQLCond.ParamCount}}
	args := make([]interface{}, 0, paramCount{{with .SQLCond.ArrIdentArgsLen}}+{{.}}{{end}})
	args = append(args, {{.SQL.Idents}}{{if .SQLCond.IdentArgs}}, {{.SQLCond.IdentArgs}}{{end}})
	{{- end }}
	{{- range .SQLCond.ArrIdentArgs }}
	var placeholders{{.}} string
	{
		placeholders := make([]string, 0, len({{.}}))
		for _, i := range {{.}} {
			paramCount++
			placeholders = append(placeholders, fmt.Sprintf("($%d)", paramCount))
			args = append(args, i)
		}
		placeholders{{.}} = strings.Join(placeholders, ", ")
	}
	{{- end }}
	_, err := d.ExecContext(ctx, "UPDATE "+t.TableName+" SET {{if eq .SQL.NumDBNames 1 -}}
	{{.SQL.DBNames -}}
	{{else -}}
	({{.SQL.DBNames}})
	{{- end}} = {{if eq .SQL.NumDBNames 1 -}}
	{{.SQL.Placeholders -}}
	{{else -}}
	({{.SQL.Placeholders}})
	{{- end}} WHERE {{.SQLCond.DBCond}};", {{if .SQLCond.ArrIdentArgs}}args...{{else}}{{.SQL.Idents}}, {{.SQLCond.IdentArgs}}{{end}})
	if err != nil {
		return err
	}
	return nil
}
`
