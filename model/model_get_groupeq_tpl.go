package model

const templateGetGroupEq = `
func (t *{{.Prefix}}ModelTable) Get{{.ModelIdent}}{{.SQLCond.IdentNames}}Ord{{.PrimaryField.Ident}}(ctx context.Context, d db.SQLExecutor, {{.SQLCond.IdentParams}}, orderasc bool, limit, offset int) (_ []{{.ModelIdent}}, retErr error) {
	{{- if .SQLCond.ArrIdentArgs }}
	paramCount := {{.SQLCond.ParamCount}}
	args := make([]interface{}, 0, paramCount{{with .SQLCond.ArrIdentArgsLen}}+{{.}}{{end}})
	args = append(args, limit, offset{{if .SQLCond.IdentArgs}}, {{.SQLCond.IdentArgs}}{{end}})
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
	rows, err := d.QueryContext(ctx, "SELECT {{.SQL.DBNames}} FROM "+t.TableName+" WHERE {{.SQLCond.DBCond}} ORDER BY {{.PrimaryField.DBName}} "+order+" LIMIT $1 OFFSET $2;", {{if .SQLCond.ArrIdentArgs}}args...{{else}}limit, offset, {{.SQLCond.IdentArgs}}{{end}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("Failed to close db rows: %w", err))
		}
	}()
	for rows.Next() {
		var m {{.ModelIdent}}
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
