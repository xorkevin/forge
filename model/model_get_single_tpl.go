package model

const templateGetOneEq = `
func {{.Prefix}}ModelGet{{.ModelIdent}}{{.SQLCond.IdentNames}}(db *sql.DB, {{.SQLCond.IdentParams}}) (*{{.ModelIdent}}, int, error) {
	{{- if .SQLCond.ArrIdentArgs }}
	paramCount := {{.SQLCond.ParamCount}}
	args := make([]interface{}, 0, paramCount{{with .SQLCond.ArrIdentArgsLen}}+{{.}}{{end}})
	args = append(args, {{.SQLCond.IdentArgs}})
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
	m := &{{.ModelIdent}}{}
	if err := db.QueryRow("SELECT {{.SQL.DBNames}} FROM {{.TableName}} WHERE {{.SQLCond.DBCond}};", {{if .SQLCond.ArrIdentArgs}}args...{{else}}{{.SQLCond.IdentArgs}}{{end}}).Scan({{.SQL.IdentRefs}}); err != nil {
		if err == sql.ErrNoRows {
			return nil, 2, err
		}
		if postgresErr, ok := err.(*pq.Error); ok {
			switch postgresErr.Code {
			case "42P01": // undefined_table
				return nil, 4, err
			default:
				return nil, 0, err
			}
		}
		return nil, 0, err
	}
	return m, 0, nil
}
`
