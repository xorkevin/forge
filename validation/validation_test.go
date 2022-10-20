package validation

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/forge/writefs"
)

//
//func (r reqSearchUsers) valid() error {
//	if err := validoptUsername(r.Prefix); err != nil {
//		return err
//	}
//	if err := validAmount(r.Amount); err != nil {
//		return err
//	}
//	return nil
//}

func TestGenerate(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	now := time.Now()
	var filemode fs.FileMode = 0644

	fsys := fstest.MapFS{
		"stuff.go": &fstest.MapFile{
			Data: []byte(`package somepackage

type (
	//forge:valid
	reqSearchUsers struct {
		Prefix string ` + "`" + `valid:"username,opt" json:"-"` + "`" + `
		Amount int    ` + "`" + `valid:"amount" json:"-"` + "`" + `
	}

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
	}

	outputfs := writefs.NewFSMock()
	assert.NoError(Generate(outputfs, fsys, Opts{
		Verbose:     false,
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
	}))
	assert.Len(outputfs.Files, 1)
	assert.Equal(`// Code generated by go generate forge validation dev; DO NOT EDIT.

package somepackage

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
`, outputfs.Files["validation_gen.go"])
}
