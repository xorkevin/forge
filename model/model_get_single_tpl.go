package model

const templateGetSingle = `
func {{.Prefix}}ModelGet{{.ModelIdent}}By{{.PrimaryField.Ident}}(db *sql.DB, key {{.PrimaryField.GoType}}) (*{{.ModelIdent}}, int, error) {
	m := &{{.ModelIdent}}{}
	if err := db.QueryRow("SELECT {{.SQL.DBNames}} FROM {{.TableName}} WHERE {{.PrimaryField.DBName}} = $1;", key).Scan({{.SQL.IdentRefs}}); err != nil {
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
