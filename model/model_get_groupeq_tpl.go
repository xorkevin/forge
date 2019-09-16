package model

const templateGetGroupEq = `
func {{.Prefix}}ModelGet{{.ModelIdent}}Eq{{.SQLCond.IdentNames}}Ord{{.PrimaryField.Ident}}(db *sql.DB, {{.SQLCond.IdentParams}}, orderasc bool, limit, offset int) ([]{{.ModelIdent}}, error) {
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
	order := "DESC"
	if orderasc {
		order = "ASC"
	}
	res := make([]{{.ModelIdent}}, 0, limit)
	rows, err := db.Query("SELECT {{.SQL.DBNames}} FROM {{.TableName}} WHERE {{.SQLCond.DBCond}} ORDER BY {{.PrimaryField.DBName}} "+order+" LIMIT $1 OFFSET $2;", limit, offset, {{if .SQLCond.ArrIdentArgs}}args{{else}}{{.SQLCond.IdentArgs}}{{end}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
		}
	}()
	for rows.Next() {
		m := {{.ModelIdent}}{}
		if err := rows.Scan({{.SQL.IdentRefs}}); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
`
