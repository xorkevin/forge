package main

const templateQuerySingle = `
func {{.Prefix}}ModelGet{{.ModelIdent}}By{{.PrimaryField.Ident}}(db *sql.DB, key {{.PrimaryField.GoType}}) (*{{.ModelIdent}}, int, error) {
	m := &{{.ModelIdent}}{}
	if err := db.QueryRow("SELECT {{.SQL.DBNames}} FROM {{.TableName}} WHERE {{.PrimaryField.DBName}} = $1;", key).Scan({{.SQL.IdentRefs}}); err != nil {
		if err == sql.ErrNoRows {
			return nil, 2, err
		}
		return nil, 0, err
	}
	return m, 0, nil
}
`
