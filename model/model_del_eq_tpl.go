package model

const templateDelEq = `
func {{.Prefix}}ModelDelEq{{.SQLCond.IdentNames}}(db *sql.DB, {{.SQLCond.IdentParams}}) error {
	_, err := db.Exec("DELETE FROM {{.TableName}} WHERE {{.SQLCond.DBCond}};", {{.SQLCond.IdentArgs}})
	return err
}
`
