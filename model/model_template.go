package model

const templateModel = `
type (
	{{.Prefix}}ModelTable struct {
		TableName string
	}
)

func (t *{{.Prefix}}ModelTable) Setup(ctx context.Context, d sqldb.Executor) error {
	_, err := d.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+t.TableName+" ({{.SQL.Setup}});")
	if err != nil {
		return err
	}
	{{- range .SQL.Indicies }}
	_, err = d.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS "+t.TableName+"_{{.Name}}_index ON "+t.TableName+" ({{.Columns}});")
	if err != nil {
		return err
	}
	{{- end }}
	return nil
}

func (t *{{.Prefix}}ModelTable) Insert(ctx context.Context, d sqldb.Executor, m *{{.ModelIdent}}) error {
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" ({{.SQL.DBNames}}) VALUES ({{.SQL.Placeholders}});", {{.SQL.Idents}})
	if err != nil {
		return err
	}
	return nil
}

func (t *{{.Prefix}}ModelTable) InsertBulk(ctx context.Context, d sqldb.Executor, models []*{{.ModelIdent}}, allowConflict bool) error {
	conflictSQL := ""
	if allowConflict {
		conflictSQL = " ON CONFLICT DO NOTHING"
	}
	placeholders := make([]string, 0, len(models))
	args := make([]interface{}, 0, len(models)*{{.SQL.ColNum}})
	for c, m := range models {
		n := c * {{.SQL.ColNum}}
		placeholders = append(placeholders, fmt.Sprintf("({{.SQL.PlaceholderTpl}})", {{.SQL.PlaceholderCount}}))
		args = append(args, {{.SQL.Idents}})
	}
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" ({{.SQL.DBNames}}) VALUES "+strings.Join(placeholders, ", ")+conflictSQL+";", args...)
	if err != nil {
		return err
	}
	return nil
}
`
