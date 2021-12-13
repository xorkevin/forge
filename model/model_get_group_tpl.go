package model

const templateGetGroup = `
func {{.Prefix}}ModelGet{{.ModelIdent}}Ord{{.PrimaryField.Ident}}(db *sql.DB, tableName string, orderasc bool, limit, offset int) ([]{{.ModelIdent}}, error) {
	order := "DESC"
	if orderasc {
		order = "ASC"
	}
	res := make([]{{.ModelIdent}}, 0, limit)
	rows, err := db.Query("SELECT {{.SQL.DBNames}} FROM "+tableName+" ORDER BY {{.PrimaryField.DBName}} "+order+" LIMIT $1 OFFSET $2;", limit, offset)
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
