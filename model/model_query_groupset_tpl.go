package model

const importsQueryGroupSet = `"strconv" "strings"`

const templateQueryGroupSet = `
func {{.Prefix}}ModelGet{{.ModelIdent}}Set{{.PrimaryField.Ident}}(db *sql.DB, keys []{{.PrimaryField.GoType}}) ([]{{.ModelIdent}}, error) {
	placeholderStart := 1
	placeholders := make([]string, 0, len(keys))
	for i := range keys {
		placeholders = append(placeholders, "($"+strconv.Itoa(i+placeholderStart)+")")
	}

	args := make([]interface{}, 0, len(keys))
	for _, i := range keys {
		args = append(args, i)
	}

	stmt := "SELECT {{.SQL.DBNames}} FROM {{.TableName}} WHERE {{.PrimaryField.DBName}} IN (VALUES " + strings.Join(placeholders, ",") + ");"

	res := make([]{{.ModelIdent}}, 0, len(keys))
	rows, err := db.Query(stmt, args...)
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
