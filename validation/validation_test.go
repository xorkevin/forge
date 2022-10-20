package validation

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
	//forge:valid
	reqSearchUsers struct {
		Prefix string ` + "`" + `valid:"username,opt" json:"-"` + "`" + `
		Amount int    ` + "`" + `valid:"amount" json:"-"` + "`" + `
	}

	//forge:validnope
	reqOther struct {
		Prefix string ` + "`" + `valid:"username,opt" json:"-"` + "`" + `
		Amount int    ` + "`" + `valid:"amount" json:"-"` + "`" + `
	}

	//forge:valid
	reqGetUsers struct {
		Userids []string ` + "`" + `valid:"userids,has" json:"-"` + "`" + `
		Foo string ` + "`" + `json:"foo"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"more.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqAnother struct {
		Prefix string ` + "`" + `valid:"username,opt" json:"-"` + "`" + `
		Amount int    ` + "`" + `valid:"amount" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff_again.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqAgain struct {
		Prefix string ` + "`" + `valid:"username,opt" json:"-"` + "`" + `
		Amount int    ` + "`" + `valid:"amount" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
				"morestuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID struct {
		Userid string ` + "`" + `valid:"userid,has" json:"-"` + "`" + `
		Other string
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Output: map[string]string{
				"validation_gen.go": `// Code generated by go generate forge validation dev; DO NOT EDIT.

package somepackage

func (r reqUserGetID) valid() error {
	if err := validhasUserid(r.Userid); err != nil {
		return err
	}
	return nil
}

func (r reqSearchUsers) valid() error {
	if err := validoptUsername(r.Prefix); err != nil {
		return err
	}
	if err := validAmount(r.Amount); err != nil {
		return err
	}
	return nil
}

func (r reqGetUsers) valid() error {
	if err := validhasUserids(r.Userids); err != nil {
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
			Name: "errors on no validations",
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
			Name: "errors on validation directive on non-typedef",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

//forge:valid
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
			Name: "errors on validation tag on multiple fields",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID struct {
		Userid, Other string ` + "`" + `valid:"userid,has" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidValidator{},
		},
		{
			Name: "errors on malformed validation tag",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID struct {
		Userid string ` + "`" + `valid:"" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidValidator{},
		},
		{
			Name: "errors on invalid validation tag value",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID struct {
		Userid string ` + "`" + `valid:"userid,bogus" json:"-"` + "`" + `
	}
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidValidator{},
		},
		{
			Name: "errors on no validation tags",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID struct {
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
			Name: "errors on validation directive on non-struct",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID []string
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
		{
			Name: "errors on validation directive on type alias",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage

type (
	//forge:valid
	reqUserGetID = string
)
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorInvalidFile{},
		},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			outputfs := writefs.NewFSMock()
			err := Generate(outputfs, tc.Fsys, Opts{
				Verbose:     true,
				Version:     "dev",
				Output:      "validation_gen.go",
				Prefix:      "valid",
				PrefixValid: "valid",
				PrefixHas:   "validhas",
				PrefixOpt:   "validopt",
				Include:     "stuff",
				Ignore:      `_again\.go$`,
				Directive:   "forge:valid",
				Tag:         "valid",
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
				Verbose:     true,
				Version:     "dev",
				Output:      "validation_gen.go",
				Prefix:      "valid",
				PrefixValid: "valid",
				PrefixHas:   "validhas",
				PrefixOpt:   "validopt",
				Include:     `\y`,
				Ignore:      "",
				Directive:   "forge:valid",
				Tag:         "valid",
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
				Verbose:     true,
				Version:     "dev",
				Output:      "validation_gen.go",
				Prefix:      "valid",
				PrefixValid: "valid",
				PrefixHas:   "validhas",
				PrefixOpt:   "validopt",
				Include:     "",
				Ignore:      `\y`,
				Directive:   "forge:valid",
				Tag:         "valid",
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
			Verbose:     true,
			Version:     "dev",
			Output:      "validation_gen.go",
			Prefix:      "valid",
			PrefixValid: "valid",
			PrefixHas:   "validhas",
			PrefixOpt:   "validopt",
			Include:     "",
			Ignore:      "",
			Directive:   "forge:valid",
			Tag:         "valid",
		}, ExecEnv{
			GoPackage: "somepackage",
		})
		assert.ErrorIs(err, gopackages.ErrorConflictingPackage{})
	})
}
