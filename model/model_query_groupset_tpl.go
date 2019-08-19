package model

const templateQueryGroupSet = `
func {{.Prefix}}ModelGet{{.ModelIdent}}Set{{.PrimaryField.Ident}}(db *sql.DB, keys []{{.PrimaryField.GoType}}) ([]{{.ModelIdent}}, error) {
	placeholderStart := 1
	placeholders := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))
	for n, i := range keys {
		placeholders = append(placeholders, fmt.Sprintf("($%d)", n+placeholderStart))
		args = append(args, i)
	}
	rows, err := db.Query("SELECT {{.SQL.DBNames}} FROM {{.TableName}} WHERE {{.PrimaryField.DBName}} IN (VALUES "+strings.Join(placeholders, ", ")+");", args...)
	if err != nil {
		return nil, err
	}
	res := make([]{{.ModelIdent}}, 0, len(keys))
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
