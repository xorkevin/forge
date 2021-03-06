package model

const templateUpdEq = `
func {{.Prefix}}ModelUpd{{.ModelIdent}}{{.SQLCond.IdentNames}}(db *sql.DB, m *{{.ModelIdent}}, {{.SQLCond.IdentParams}}) (int, error) {
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
	_, err := db.Exec("UPDATE {{.TableName}} SET ({{.SQL.DBNames}}) = ROW({{.SQL.Placeholders}}) WHERE {{.SQLCond.DBCond}};", {{if .SQLCond.ArrIdentArgs}}args...{{else}}{{.SQL.Idents}}, {{.SQLCond.IdentArgs}}{{end}})
	if err != nil {
		if postgresErr, ok := err.(*pq.Error); ok {
			switch postgresErr.Code {
			case "23505": // unique_violation
				return 3, err
			default:
				return 0, err
			}
		}
	}
	return 0, nil
}
`
