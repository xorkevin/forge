package model

const templateDelGroupSet = `
func {{.Prefix}}ModelDelSet{{.PrimaryField.Ident}}(db *sql.DB, keys []{{.PrimaryField.GoType}}) error {
	placeholderStart := 1
	placeholders := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))
	for n, i := range keys {
		placeholders = append(placeholders, fmt.Sprintf("($%d)", n+placeholderStart))
		args = append(args, i)
	}
	_, err := db.Exec("DELETE FROM {{.TableName}} WHERE {{.PrimaryField.DBName}} IN (VALUES "+strings.Join(placeholders, ", ")+");", args...)
	return err
}
`
