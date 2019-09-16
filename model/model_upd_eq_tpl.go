package model

const templateUpdEq = `
func {{.Prefix}}ModelUpdate{{.ModelIdent}}Eq{{.SQLCond.IdentNames}}(db *sql.DB, m *{{.ModelIdent}}, {{.SQLCond.IdentParams}}) error {
	{{- if .SQLCond.ArrIdentArgs }}
	paramCount := {{.SQLCond.ParamCount}}
	args := make([]interface{}, 0, paramCount{{with .SQLCond.ArrIdentArgsLen}}+{{.}}{{end}})
	args = append(args, {{.SQL.Idents}}, {{.SQLCond.IdentArgs}})
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
	_, err := db.Exec("UPDATE {{.TableName}} SET ({{.SQL.DBNames}}) = ({{.SQL.Placeholders}}) WHERE {{.SQLCond.DBCond}};", {{if .SQLCond.ArrIdentArgs}}args{{else}}{{.SQL.Idents}}, {{.SQLCond.IdentArgs}}{{end}})
	return err
}
`
