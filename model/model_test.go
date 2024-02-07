package model

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/forge/gopackages"
	"xorkevin.dev/kfs/kfstest"
	"xorkevin.dev/klog"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var filemode fs.FileMode = 0o644

	for _, tc := range []struct {
		Name   string
		Fsys   fs.FS
		Output map[string]string
		Err    error
	}{
		{
			Name: "parses directives from files",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "model": {
        "setup": "UNIQUE (first_name)",
        "constraints": [
          {
            "kind": "UNIQUE",
            "columns": ["username", "first_name"]
          }
        ],
        "indicies": [
          {
            "name": "names",
            "columns": [{"col": "first_name"}, {"col": "username", "dir": "DESC"}]
          }
        ]
      },
      "queries": {
        "Model": [
          {
            "kind": "getoneeq",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ]
          },
          {
            "kind": "deleq",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ]
          },
          {
            "kind": "getoneeq",
            "name": "ByUsername",
            "conditions": [
              {"col": "username"}
            ]
          }
        ],
        "userProps": [
          {
            "kind": "updeq",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ]
          }
        ],
        "usernameProps": [
          {
            "kind": "updeq",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ]
          }
        ],
        "Info": [
          {
            "kind": "getgroup",
            "name": "All",
            "order": [
              {"col": "userid", "dir": "DESC"}
            ]
          },
          {
            "kind": "getgroupeq",
            "name": "ByIDs",
            "conditions": [
              {"col": "userid", "cond": "in"}
            ],
            "order": [
              {"col": "userid"}
            ]
          },
          {
            "kind": "getgroupeq",
            "name": "LikeUsername",
            "conditions": [
              {"col": "username", "cond": "like"}
            ],
            "order": [
              {"col": "username"}
            ]
          }
        ],
        "InfoAgain": [
          {
            "kind": "getgroup",
            "name": "All",
            "order": [
              {"col": "userid"}
            ]
          },
          {
            "kind": "getgroupeq",
            "name": "ByIDs",
            "conditions": [
              {"col": "userid", "cond": "in"}
            ],
            "order": [
              {"col": "userid"}
            ]
          },
          {
            "kind": "getgroupeq",
            "name": "LikeUsername",
            "conditions": [
              {"col": "username", "cond": "like"}
            ],
            "order": [
              {"col": "username"}
            ]
          }
        ]
      }
    },
    "sm": {
      "queries": {
        "SM": [
          {
            "kind": "getoneeq",
            "name": "ManyCond",
            "conditions": [
              {"col": "userid", "cond": "neq"},
              {"col": "username", "cond": "lt"},
              {"col": "first_name", "cond": "leq"},
              {"col": "last_name", "cond": "gt"},
              {"col": "email", "cond": "geq"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
		Username string ` + "`" + `model:"username,VARCHAR(255) NOT NULL UNIQUE"` + "`" + `
		FirstName string ` + "`" + `model:"first_name,VARCHAR(255) NOT NULL"` + "`" + `
	}

	//forge:model:query user
	userProps struct {
		Username string ` + "`" + `model:"username"` + "`" + `
		FirstName string ` + "`" + `model:"first_name"` + "`" + `
	}

	//forge:model:query user
	usernameProps struct {
		Username string ` + "`" + `model:"username"` + "`" + `
	}

	//forge:modelnope
	reqOther struct {
		Prefix string
		Amount int
	}

	//forge:model sm
	//forge:model:query sm
	SM struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
		Username string ` + "`" + `model:"username,VARCHAR(255)"` + "`" + `
		FirstName string ` + "`" + `model:"first_name,VARCHAR(255)"` + "`" + `
		LastName string ` + "`" + `model:"last_name,VARCHAR(255)"` + "`" + `
		Email string ` + "`" + `model:"email,VARCHAR(255)"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"morestuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type(
	//forge:model:query user
	Info struct {
		Userid string ` + "`" + `model:"userid"` + "`" + `
		Username string ` + "`" + `model:"username"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff_again.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type(
	//forge:model:query user
	InfoAgain struct {
		Userid string ` + "`" + `model:"userid"` + "`" + `
		Username string ` + "`" + `model:"username"` + "`" + `
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
	"errors"
	"fmt"
	"strings"

	"xorkevin.dev/forge/model/sqldb"
)

type (
	userModelTable struct {
		TableName string
	}
)

func (t *userModelTable) Setup(ctx context.Context, d sqldb.Executor) error {
	_, err := d.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+t.TableName+" (userid VARCHAR(31) PRIMARY KEY, username VARCHAR(255) NOT NULL UNIQUE, first_name VARCHAR(255) NOT NULL, UNIQUE (username, first_name), UNIQUE (first_name));")
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS "+t.TableName+"_names_index ON "+t.TableName+" (first_name, username DESC);")
	if err != nil {
		return err
	}
	return nil
}

func (t *userModelTable) Insert(ctx context.Context, d sqldb.Executor, m *Model) error {
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" (userid, username, first_name) VALUES ($1, $2, $3);", m.Userid, m.Username, m.FirstName)
	if err != nil {
		return err
	}
	return nil
}

func (t *userModelTable) InsertBulk(ctx context.Context, d sqldb.Executor, models []*Model, allowConflict bool) error {
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

func (t *userModelTable) GetInfoAll(ctx context.Context, d sqldb.Executor, limit, offset int) (_ []Info, retErr error) {
	res := make([]Info, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT userid, username FROM "+t.TableName+" ORDER BY userid DESC LIMIT $1 OFFSET $2;", limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("Failed to close db rows: %w", err))
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

func (t *userModelTable) GetInfoByIDs(ctx context.Context, d sqldb.Executor, userids []string, limit, offset int) (_ []Info, retErr error) {
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
	res := make([]Info, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT userid, username FROM "+t.TableName+" WHERE userid IN (VALUES "+placeholdersuserids+") ORDER BY userid LIMIT $1 OFFSET $2;", args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("Failed to close db rows: %w", err))
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

func (t *userModelTable) GetInfoLikeUsername(ctx context.Context, d sqldb.Executor, usernamePrefix string, limit, offset int) (_ []Info, retErr error) {
	res := make([]Info, 0, limit)
	rows, err := d.QueryContext(ctx, "SELECT userid, username FROM "+t.TableName+" WHERE username LIKE $3 ORDER BY username LIMIT $1 OFFSET $2;", limit, offset, usernamePrefix)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("Failed to close db rows: %w", err))
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

func (t *userModelTable) GetModelByID(ctx context.Context, d sqldb.Executor, userid string) (*Model, error) {
	m := &Model{}
	if err := d.QueryRowContext(ctx, "SELECT userid, username, first_name FROM "+t.TableName+" WHERE userid = $1;", userid).Scan(&m.Userid, &m.Username, &m.FirstName); err != nil {
		return nil, err
	}
	return m, nil
}

func (t *userModelTable) DelByID(ctx context.Context, d sqldb.Executor, userid string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM "+t.TableName+" WHERE userid = $1;", userid)
	return err
}

func (t *userModelTable) GetModelByUsername(ctx context.Context, d sqldb.Executor, username string) (*Model, error) {
	m := &Model{}
	if err := d.QueryRowContext(ctx, "SELECT userid, username, first_name FROM "+t.TableName+" WHERE username = $1;", username).Scan(&m.Userid, &m.Username, &m.FirstName); err != nil {
		return nil, err
	}
	return m, nil
}

func (t *userModelTable) UpduserPropsByID(ctx context.Context, d sqldb.Executor, m *userProps, userid string) error {
	_, err := d.ExecContext(ctx, "UPDATE "+t.TableName+" SET (username, first_name) = ($1, $2) WHERE userid = $3;", m.Username, m.FirstName, userid)
	if err != nil {
		return err
	}
	return nil
}

func (t *userModelTable) UpdusernamePropsByID(ctx context.Context, d sqldb.Executor, m *usernameProps, userid string) error {
	_, err := d.ExecContext(ctx, "UPDATE "+t.TableName+" SET username = $1 WHERE userid = $2;", m.Username, userid)
	if err != nil {
		return err
	}
	return nil
}

type (
	smModelTable struct {
		TableName string
	}
)

func (t *smModelTable) Setup(ctx context.Context, d sqldb.Executor) error {
	_, err := d.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+t.TableName+" (userid VARCHAR(31) PRIMARY KEY, username VARCHAR(255), first_name VARCHAR(255), last_name VARCHAR(255), email VARCHAR(255));")
	if err != nil {
		return err
	}
	return nil
}

func (t *smModelTable) Insert(ctx context.Context, d sqldb.Executor, m *SM) error {
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" (userid, username, first_name, last_name, email) VALUES ($1, $2, $3, $4, $5);", m.Userid, m.Username, m.FirstName, m.LastName, m.Email)
	if err != nil {
		return err
	}
	return nil
}

func (t *smModelTable) InsertBulk(ctx context.Context, d sqldb.Executor, models []*SM, allowConflict bool) error {
	conflictSQL := ""
	if allowConflict {
		conflictSQL = " ON CONFLICT DO NOTHING"
	}
	placeholders := make([]string, 0, len(models))
	args := make([]interface{}, 0, len(models)*5)
	for c, m := range models {
		n := c * 5
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4, n+5))
		args = append(args, m.Userid, m.Username, m.FirstName, m.LastName, m.Email)
	}
	_, err := d.ExecContext(ctx, "INSERT INTO "+t.TableName+" (userid, username, first_name, last_name, email) VALUES "+strings.Join(placeholders, ", ")+conflictSQL+";", args...)
	if err != nil {
		return err
	}
	return nil
}

func (t *smModelTable) GetSMManyCond(ctx context.Context, d sqldb.Executor, userid string, username string, firstname string, lastname string, email string) (*SM, error) {
	m := &SM{}
	if err := d.QueryRowContext(ctx, "SELECT userid, username, first_name, last_name, email FROM "+t.TableName+" WHERE userid <> $1 AND username < $2 AND first_name <= $3 AND last_name > $4 AND email >= $5;", userid, username, firstname, lastname, email).Scan(&m.Userid, &m.Username, &m.FirstName, &m.LastName, &m.Email); err != nil {
		return nil, err
	}
	return m, nil
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
			Err: ErrEnv,
		},
		{
			Name: "errors on invalid schema file",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data:    []byte(`"bogus"`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidSchema,
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
			Err: ErrInvalidFile,
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
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on model directive without prefix arg",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on model tag on multiple fields",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid, Other string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on malformed model tag",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:""` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
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
			Err: ErrInvalidModel,
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
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on duplicate model field",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
		Username string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid model index opt field",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "model": {
        "indicies": [
          {
            "columns": [{"col": "bogus"}, {"col": "userid"}]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on missing model index opt columns",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "model": {
        "indicies": [
          {
            "columns": []
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on missing model constraint opt kind",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "model": {
        "constraints": [
          {
            "kind": "",
            "columns": ["userid"]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on missing model constraint opt columns",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "model": {
        "constraints": [
          {
            "kind": "UNIQUE",
            "columns": []
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid model constraint opt field",
			Fsys: fstest.MapFS{
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "model": {
        "constraints": [
          {
            "kind": "UNIQUE",
            "columns": ["bogus", "userid"]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on query directive without prefix arg",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on query directive without model def",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query dne
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on query directive on non-typedef",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)

//forge:model:query user
const (
	foo = "bar"
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on query directive on non-struct",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}

	//forge:model:query user
	Info []string
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on query without fields",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}

	//forge:model:query user
	Info struct {
		Userid string
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on query tag on multiple fields",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}

  //forge:model:query user
	Info struct {
		Userid, Other string ` + "`" + `model:"userid"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidFile,
		},
		{
			Name: "errors on malformed query tag",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}

	//forge:model:query user
	Info struct {
		Userid string ` + "`" + `model:""` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid query tag field",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}

	//forge:model:query user
	Info struct {
		Userid string ` + "`" + `model:"bogus"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on no queries",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data:    []byte(`{}`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on missing query name",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getoneeq",
            "name": "",
            "conditions": [
              {"col": "userid"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid query kind",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "bogus",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on missing required query conditions",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getoneeq",
            "name": "ByID"
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on providing conditions when not accepted",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getgroup",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid query cond",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getoneeq",
            "name": "ByID",
            "conditions": [
              {"col": "userid", "cond": "bogus"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid query cond field",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getoneeq",
            "name": "ByID",
            "conditions": [
              {"col": "bogus"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on providing query order when not accepted",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getoneeq",
            "name": "ByID",
            "conditions": [
              {"col": "userid"}
            ],
            "order": [
              {"col": "userid"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
		{
			Name: "errors on invalid query order field",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:model user
	//forge:model:query user
	Model struct {
		Userid string ` + "`" + `model:"userid,VARCHAR(31) PRIMARY KEY"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"model.json": &fstest.MapFile{
					Data: []byte(`
{
  "models": {
    "user": {
      "queries": {
        "Model": [
          {
            "kind": "getgroup",
            "name": "All",
            "order": [
              {"col": "bogus"}
            ]
          }
        ]
      }
    }
  }
}
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrInvalidModel,
		},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			outputfs := &kfstest.MapFS{
				Fsys: fstest.MapFS{},
			}
			err := Generate(context.Background(), klog.Discard{}, outputfs, tc.Fsys, "dev", Opts{
				Output:            "model_gen.go",
				Schema:            "model.json",
				Include:           "stuff",
				Ignore:            `_again\.go$`,
				ModelDirective:    "forge:model",
				QueryDirective:    "forge:model:query",
				ModelTag:          "model",
				PlaceholderPrefix: "$",
			}, ExecEnv{
				GoPackage: "somepackage",
			})
			if err != nil {
				assert.ErrorIs(err, tc.Err)
				return
			}
			assert.NoError(err)
			assert.Len(outputfs.Fsys, len(tc.Output))
			for k, v := range tc.Output {
				assert.Equal(v, string(outputfs.Fsys[k].Data))
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

			outputfs := &kfstest.MapFS{
				Fsys: fstest.MapFS{},
			}
			err := Generate(context.Background(), klog.Discard{}, outputfs, fsys, "dev", Opts{
				Output:            "model_gen.go",
				Schema:            "model.json",
				Include:           `\y`,
				Ignore:            `_again\.go$`,
				ModelDirective:    "forge:model",
				QueryDirective:    "forge:model:query",
				ModelTag:          "model",
				PlaceholderPrefix: "$",
			}, ExecEnv{
				GoPackage: "somepackage",
			})
			assert.Error(err)
		})

		t.Run("invalid ignore", func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			outputfs := &kfstest.MapFS{
				Fsys: fstest.MapFS{},
			}
			err := Generate(context.Background(), klog.Discard{}, outputfs, fsys, "dev", Opts{
				Output:            "model_gen.go",
				Schema:            "model.json",
				Include:           "stuff",
				Ignore:            `\y`,
				ModelDirective:    "forge:model",
				QueryDirective:    "forge:model:query",
				ModelTag:          "model",
				PlaceholderPrefix: "$",
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

		outputfs := &kfstest.MapFS{
			Fsys: fstest.MapFS{},
		}
		err := Generate(context.Background(), klog.Discard{}, outputfs, fsys, "dev", Opts{
			Output:            "model_gen.go",
			Schema:            "model.json",
			Include:           "",
			Ignore:            "",
			ModelDirective:    "forge:model",
			QueryDirective:    "forge:model:query",
			ModelTag:          "model",
			PlaceholderPrefix: "$",
		}, ExecEnv{
			GoPackage: "somepackage",
		})
		assert.ErrorIs(err, gopackages.ErrorConflictingPackage{})
	})
}

func TestQueryKindString(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	for _, tc := range []struct {
		Kind   queryKind
		String string
	}{
		{
			Kind:   queryKindGetOneEq,
			String: "getoneeq",
		},
		{
			Kind:   queryKindGetGroup,
			String: "getgroup",
		},
		{
			Kind:   queryKindGetGroupEq,
			String: "getgroupeq",
		},
		{
			Kind:   queryKindUpdEq,
			String: "updeq",
		},
		{
			Kind:   queryKindDelEq,
			String: "deleq",
		},
		{
			Kind:   queryKindUnknown,
			String: "unknown",
		},
	} {
		tc := tc
		assert.Equal(tc.String, tc.Kind.String())
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	for _, tc := range []struct {
		Err    error
		String string
	}{
		{
			Err:    ErrEnv,
			String: "Invalid execution environment",
		},
		{
			Err:    ErrInvalidSchema,
			String: "Invalid schema",
		},
		{
			Err:    ErrInvalidFile,
			String: "Invalid file",
		},
		{
			Err:    ErrInvalidModel,
			String: "Invalid model",
		},
	} {
		tc := tc
		assert.Equal(tc.String, tc.Err.Error())
	}
}
