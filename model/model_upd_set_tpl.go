package model

const templateUpdSet = `
func {{.Prefix}}ModelUpdate{{.ModelIdent}}Set{{.PrimaryField.Ident}}(db *sql.DB, m *{{.ModelIdent}}, keys []{{.PrimaryField.GoType}}) error {
	placeholderStart := {{.SQL.ColNum}}
	placeholders := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys)+placeholderStart)
	args = append(args, {{.SQL.Idents}})
	for n, i := range keys {
		placeholders = append(placeholders, fmt.Sprintf("($%d)", n+placeholderStart+1))
		args = append(args, i)
	}
	_, err := db.Exec("UPDATE {{.TableName}} SET ({{.SQL.DBNames}}) = ({{.SQL.Placeholders}}) WHERE {{.PrimaryField.DBName}} IN (VALUES "+strings.Join(placeholders, ", ")+";", args...)
	return err
}
`
