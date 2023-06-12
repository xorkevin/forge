package model

const templateGetGroup = `
func (t *{{.Prefix}}ModelTable) Get{{.ModelIdent}}Ord{{.PrimaryField.Ident}}(ctx context.Context, d sqldb.Executor, orderasc bool, limit, offset int) (_ []{{.ModelIdent}}, retErr error) {
	order := "DESC"
	if orderasc {
		order = "ASC"
	}
	res := make([]{{.ModelIdent}}, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT {{.SQL.DBNames}} FROM "+t.TableName+" ORDER BY {{.PrimaryField.DBName}} "+order+" LIMIT $1 OFFSET $2;", limit, offset)
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
