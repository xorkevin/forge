package gopackages

import (
	"go/ast"
	"io/fs"
	"regexp"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadDir(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var filemode fs.FileMode = 0644

	for _, tc := range []struct {
		Name    string
		PkgName string
		Fsys    fs.FS
		Include *regexp.Regexp
		Ignore  *regexp.Regexp
		Files   []string
		Err     error
	}{
		{
			Name:    "selects go source files",
			PkgName: "somepackage",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
				"more_stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff_test.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
				"file.txt": &fstest.MapFile{
					Data: []byte(`plain text file
`),
					Mode:    filemode,
					ModTime: now,
				},
				"linkto.go": &fstest.MapFile{
					Data: []byte(`stuff.go
`),
					Mode:    filemode | fs.ModeSymlink,
					ModTime: now,
				},
				"subdir/sub.go": &fstest.MapFile{
					Data: []byte(`package subdir
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Files: []string{"stuff.go", "more_stuff.go"},
		},
		{
			Name:    "handles regex include and ignore",
			PkgName: "somepackage",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
				"more_stuff.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
				"stuff_gen.go": &fstest.MapFile{
					Data: []byte(`package somepackage
`),
					Mode:    filemode,
					ModTime: now,
				},
				"other.go": &fstest.MapFile{
					Data: []byte(`bogus
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Include: regexp.MustCompile(`stuff`),
			Ignore:  regexp.MustCompile(`_gen\.go$`),
			Files:   []string{"stuff.go", "more_stuff.go"},
		},
		{
			Name: "errors on go file parse error",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`bogus
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorParseFile{},
		},
		{
			Name: "errors on conflicting package name",
			Fsys: fstest.MapFS{
				"stuff.go": &fstest.MapFile{
					Data: []byte(`package foo
`),
					Mode:    filemode,
					ModTime: now,
				},
				"other.go": &fstest.MapFile{
					Data: []byte(`package bar
`),
					Mode:    filemode,
					ModTime: now,
				},
			},
			Err: ErrorConflictingPackage{},
		},
	} {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			pkg, err := ReadDir(tc.Fsys, tc.Include, tc.Ignore)
			if tc.Err != nil {
				assert.Error(err)
				assert.ErrorIs(err, tc.Err)
				return
			}
			assert.NoError(err)

			assert.Len(pkg.Files, len(tc.Files))
			for _, i := range tc.Files {
				assert.NotNil(pkg.Files[i])
			}
		})
	}
}

func TestFindDirectives(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	now := time.Now()
	var filemode fs.FileMode = 0644

	fsys := fstest.MapFS{
		"stuff.go": &fstest.MapFile{
			Data: []byte(`// comment before package

// Package somepackage does stuff
package somepackage

// comment after package

import (
	"fmt"
)

// comment before type
//forge:abc dir3
type (
	// comment before foo

	// Foo is a struct
	//forge:abc dir1
	Foo struct {
		// comment before bar in foo

		// Bar is something
		//forge:abc dir2
		Bar string // comment after bar
	}

	// comment after foo
)

// constants here
//forge:bc dir4
//forge:abc dir5
const (
	// comment inside constants

	// abc doc comment
	//forge:abc dir6
	abc = "123" // comment after abc
)

// comment before string func

// String implements stringer
//
//forge:abc dir7
func (f *Foo) String() string {
	// comment in string
	return f.Bar
}

// vars here
//forge:bc dir9
/*
forge:abc dir10
*/
var (
	// comment inside constants

	// abc doc comment
	//forge:abc dir8
	someFoo = Foo{
		Bar: "bar",
	}
)
`),
			Mode:    filemode,
			ModTime: now,
		},
	}

	astfiles, err := ReadDir(fsys, nil, nil)
	assert.NoError(err)

	dirs := FindDirectives(astfiles, []string{"forge:abc", "forge:bc"})
	assert.Len(dirs, 4)
	for n, tc := range []struct {
		Directives []DirectiveInstance
		Kind       ObjKind
		Name       string
	}{
		{
			Directives: []DirectiveInstance{
				{
					Sigil:     "forge:abc",
					Directive: "forge:abc dir3",
				},
			},
			Kind: ObjKindGroupType,
			Name: "",
		},
		{
			Directives: []DirectiveInstance{
				{
					Sigil:     "forge:abc",
					Directive: "forge:abc dir1",
				},
			},
			Kind: ObjKindDeclType,
			Name: "Foo",
		},
		{
			Directives: []DirectiveInstance{
				{
					Sigil:     "forge:bc",
					Directive: "forge:bc dir4",
				},
				{
					Sigil:     "forge:abc",
					Directive: "forge:abc dir5",
				},
			},
			Kind: ObjKindGroupConst,
			Name: "",
		},
		{
			Directives: []DirectiveInstance{
				{
					Sigil:     "forge:bc",
					Directive: "forge:bc dir9",
				},
			},
			Kind: ObjKindGroupVar,
			Name: "",
		},
	} {
		tc := tc

		assert.Equal(tc.Directives, dirs[n].Directives)
		assert.Equal(tc.Kind, dirs[n].Kind)
		switch tc.Kind {
		case ObjKindDeclType:
			{
				declType, ok := dirs[n].Obj.(*ast.TypeSpec)
				assert.True(ok)
				assert.NotNil(declType.Name)
				assert.Equal(tc.Name, declType.Name.Name)
			}
		}
	}
}
