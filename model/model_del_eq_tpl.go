package model

const templateDelEq = `
func {{.Prefix}}ModelDelEq{{.SQLCond.IdentNames}}(db *sql.DB, {{.SQLCond.IdentParams}}) error {
	{{- if .SQLCond.ArrIdentArgs }}
	paramCount := {{.SQLCond.ParamCount}}
	args := make([]interface{}, 0, paramCount{{with .SQLCond.ArrIdentArgsLen}}+{{.}}{{end}})
	{{- if .SQLCond.IdentArgs }}
	args = append(args, {{.SQLCond.IdentArgs}})
	{{- end }}
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
	_, err := db.Exec("DELETE FROM {{.TableName}} WHERE {{.SQLCond.DBCond}};", {{if .SQLCond.ArrIdentArgs}}args{{else}}{{.SQLCond.IdentArgs}}{{end}})
	return err
}
`
