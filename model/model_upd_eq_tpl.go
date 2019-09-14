package model

const templateUpdEq = `
func {{.Prefix}}ModelUpdate{{.ModelIdent}}Eq{{.SQLCond.IdentNames}}(db *sql.DB, m *{{.ModelIdent}}, {{.SQLCond.IdentParams}}) error {
	_, err := db.Exec("UPDATE {{.TableName}} SET ({{.SQL.DBNames}}) = ({{.SQL.Placeholders}}) WHERE {{.SQLCond.DBCond}};", {{.SQL.Idents}}, {{.SQLCond.IdentArgs}})
	return err
}
`
