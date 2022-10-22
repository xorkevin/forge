package model

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/forge/gopackages"
	"xorkevin.dev/forge/writefs"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var filemode fs.FileMode = 0644

	for _, tc := range []struct {
		Name   string
		Fsys   fs.FS
		Output map[string]string
		Err    error
	}{
		{
			Name: "parses directives from files",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY" query:"userid;getoneeq,userid;deleq,userid|eq"` + "`" + `
		Username string ` + "`" + `model:"username,VARCHAR(255) NOT NULL UNIQUE;index,first_name" query:"username;getoneeq,username"` + "`" + `
		FirstName string ` + "`" + `model:"first_name,VARCHAR(255) NOT NULL" query:"first_name"` + "`" + `
	}

	//forge:query user
	userProps struct {
		Username string ` + "`" + `query:"username;updeq,userid"` + "`" + `
		FirstName string ` + "`" + `query:"first_name"` + "`" + `
	}

	//forge:modelnope
	reqOther struct {
		Prefix string
		Amount int
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"morestuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type(
	//forge:query user
	Info struct {
		Userid string ` + "`" + `query:"userid;getgroup;getgroupeq,userid|in"` + "`" + `
		Username string ` + "`" + `query:"username;getgroupeq,username|like"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff_again.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type(
	//forge:query user
	InfoAgain struct {
		Userid string ` + "`" + `query:"userid;getgroup;getgroupeq,userid|in"` + "`" + `
		Username string ` + "`" + `query:"username;getgroupeq,username|like"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Output: map[string]string{
				"model_gen.go": `// Code generated by go generate forge model dev; DO NOT EDIT.

package somepackage

import (
	"context"
	"fmt"
	"strings"
	"xorkevin.dev/governor/service/db"
)

type (
	userModelTable struct {
		TableName string
	}
)

func (t *userModelTable) Setup(ctx context.Context, d db.SQLExecutor) error {
	_, err := d.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+t.TableName+" (userid VARCHAR(31) PRIMARY KEY, username VARCHAR(255) NOT NULL UNIQUE, first_name VARCHAR(255) NOT NULL);")
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS "+t.TableName+"_first_name__username_index ON "+t.TableName+" (first_name, username);")
	if err != nil {
		return err
	}
	return nil
}

func (t *userModelTable) Insert(ctx context.Context, d db.SQLExecutor, m *Model) error {
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" (userid, username, first_name) VALUES ($1, $2, $3);", m.Userid, m.Username, m.FirstName)
	if err != nil {
		return err
	}
	return nil
}

func (t *userModelTable) InsertBulk(ctx context.Context, d db.SQLExecutor, models []*Model, allowConflict bool) error {
	conflictSQL := ""
	if allowConflict {
		conflictSQL = " ON CONFLICT DO NOTHING"
	}
	placeholders := make([]string, 0, len(models))
	args := make([]interface{}, 0, len(models)*3)
	for c, m := range models {
		n := c * 3
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d)", n+1, n+2, n+3))
		args = append(args, m.Userid, m.Username, m.FirstName)
	}
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" (userid, username, first_name) VALUES "+strings.Join(placeholders, ", ")+conflictSQL+";", args...)
	if err != nil {
		return err
	}
	return nil
}

func (t *userModelTable) GetInfoOrdUserid(ctx context.Context, d db.SQLExecutor, orderasc bool, limit, offset int) ([]Info, error) {
	order := "DESC"
	if orderasc {
		order = "ASC"
	}
	res := make([]Info, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT userid, username FROM "+t.TableName+" ORDER BY userid "+order+" LIMIT $1 OFFSET $2;", limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
		}
	}()
	for rows.Next() {
		var m Info
		if err := rows.Scan(&m.Userid, &m.Username); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (t *userModelTable) GetInfoHasUseridOrdUserid(ctx context.Context, d db.SQLExecutor, userids []string, orderasc bool, limit, offset int) ([]Info, error) {
	paramCount := 2
	args := make([]interface{}, 0, paramCount+len(userids))
	args = append(args, limit, offset)
	var placeholdersuserids string
	{
		placeholders := make([]string, 0, len(userids))
		for _, i := range userids {
			paramCount++
			placeholders = append(placeholders, fmt.Sprintf("($%d)", paramCount))
			args = append(args, i)
		}
		placeholdersuserids = strings.Join(placeholders, ", ")
	}
	order := "DESC"
	if orderasc {
		order = "ASC"
	}
	res := make([]Info, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT userid, username FROM "+t.TableName+" WHERE userid IN (VALUES "+placeholdersuserids+") ORDER BY userid "+order+" LIMIT $1 OFFSET $2;", args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
		}
	}()
	for rows.Next() {
		var m Info
		if err := rows.Scan(&m.Userid, &m.Username); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (t *userModelTable) GetInfoLikeUsernameOrdUsername(ctx context.Context, d db.SQLExecutor, usernamePrefix string, orderasc bool, limit, offset int) ([]Info, error) {
	order := "DESC"
	if orderasc {
		order = "ASC"
	}
	res := make([]Info, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT userid, username FROM "+t.TableName+" WHERE username LIKE $3 ORDER BY username "+order+" LIMIT $1 OFFSET $2;", limit, offset, usernamePrefix)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
		}
	}()
	for rows.Next() {
		var m Info
		if err := rows.Scan(&m.Userid, &m.Username); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (t *userModelTable) GetModelEqUserid(ctx context.Context, d db.SQLExecutor, userid string) (*Model, error) {
	m := &Model{}
	if err := d.QueryRowContext(ctx, "SELECT userid, username, first_name FROM "+t.TableName+" WHERE userid = $1;", userid).Scan(&m.Userid, &m.Username, &m.FirstName); err != nil {
		return nil, err
	}
	return m, nil
}

func (t *userModelTable) DelEqUserid(ctx context.Context, d db.SQLExecutor, userid string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM "+t.TableName+" WHERE userid = $1;", userid)
	return err
}

func (t *userModelTable) GetModelEqUsername(ctx context.Context, d db.SQLExecutor, username string) (*Model, error) {
	m := &Model{}
	if err := d.QueryRowContext(ctx, "SELECT userid, username, first_name FROM "+t.TableName+" WHERE username = $1;", username).Scan(&m.Userid, &m.Username, &m.FirstName); err != nil {
		return nil, err
	}
	return m, nil
}

func (t *userModelTable) UpduserPropsEqUserid(ctx context.Context, d db.SQLExecutor, m *userProps, userid string) error {
	_, err := d.ExecContext(ctx, "UPDATE "+t.TableName+" SET (username, first_name) = ROW($1, $2) WHERE userid = $3;", m.Username, m.FirstName, userid)
	if err != nil {
		return err
	}
	return nil
}
`,
			},
		},
		{
			Name: "errors on wrong package",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package different
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorEnv{},
		},
		{
			Name: "errors on no models",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
		{
			Name: "errors on model directive on non-typedef",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

//forge:model user
const (
	foo = "bar"
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
		{
			Name: "errors on model directive without prefix arg",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
		{
			Name: "errors on model tag on multiple fields",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid, Other string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidModel{},
		},
		{
			Name: "errors on malformed model tag",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidModel{},
		},
		{
			Name: "errors on invalid model tag value",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY;bogus" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidModel{},
		},
		{
			Name: "errors on no model tags",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
		{
			Name: "errors on model directive on non-struct",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model []string
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
		{
			Name: "errors on duplicate model field",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY" json:"-"` + "`" + `
		Username string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidModel{},
		},
		{
			Name: "errors on invalid model index opt field",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY;index,bogus" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidModel{},
		},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			outputfs := writefs.NewFSMock()
			err := Generate(outputfs, tc.Fsys, Opts{
				Verbose:        true,
				Version:        "dev",
				Output:         "model_gen.go",
				Include:        "stuff",
				Ignore:         `_again\.go$`,
				ModelDirective: "forge:model",
				QueryDirective: "forge:query",
				ModelTag:       "model",
				QueryTag:       "query",
			}, ExecEnv{
				GoPackage: "somepackage",
			})
			if err != nil {
				assert.ErrorIs(err, tc.Err)
				return
			}
			assert.NoError(err)
			assert.Len(outputfs.Files, len(tc.Output))
			for k, v := range tc.Output {
				assert.Equal(v, outputfs.Files[k])
			}
		})
	}

	t.Run("errors on invalid regex", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"stuff.go": &fstest.MapFile{
				Data: []byte(`package somepackage
`),
				Mode:    filemode,
				ModTime: now,
			},
		}

		t.Run("invalid include", func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			outputfs := writefs.NewFSMock()
			err := Generate(outputfs, fsys, Opts{
				Verbose:        true,
				Version:        "dev",
				Output:         "model_gen.go",
				Include:        `\y`,
				Ignore:         `_again\.go$`,
				ModelDirective: "forge:model",
				QueryDirective: "forge:query",
				ModelTag:       "model",
				QueryTag:       "query",
			}, ExecEnv{
				GoPackage: "somepackage",
			})
			assert.Error(err)
		})

		t.Run("invalid ignore", func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			outputfs := writefs.NewFSMock()
			err := Generate(outputfs, fsys, Opts{
				Verbose:        true,
				Version:        "dev",
				Output:         "model_gen.go",
				Include:        "stuff",
				Ignore:         `\y`,
				ModelDirective: "forge:model",
				QueryDirective: "forge:query",
				ModelTag:       "model",
				QueryTag:       "query",
			}, ExecEnv{
				GoPackage: "somepackage",
			})
			assert.Error(err)
		})
	})

	t.Run("reports ReadDir errors", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		fsys := fstest.MapFS{
			"stuff.go": &fstest.MapFile{
				Data: []byte(`package somepackage
`),
				Mode:    filemode,
				ModTime: now,
			},
			"other.go": &fstest.MapFile{
				Data: []byte(`package different
`),
				Mode:    filemode,
				ModTime: now,
			},
		}

		outputfs := writefs.NewFSMock()
		err := Generate(outputfs, fsys, Opts{
			Verbose:        true,
			Version:        "dev",
			Output:         "model_gen.go",
			Include:        "",
			Ignore:         "",
			ModelDirective: "forge:model",
			QueryDirective: "forge:query",
			ModelTag:       "model",
			QueryTag:       "query",
		}, ExecEnv{
			GoPackage: "somepackage",
		})
		assert.ErrorIs(err, gopackages.ErrorConflictingPackage{})
	})
}
