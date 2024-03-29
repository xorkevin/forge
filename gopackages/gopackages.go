package gopackages

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"xorkevin.dev/kerrors"
)

type (
	ErrorParseFile          struct{}
	ErrorConflictingPackage struct{}
)

func (e ErrorParseFile) Error() string {
	return "Failed parsing file"
}

func (e ErrorConflictingPackage) Error() string {
	return "Conflicting package names"
}

func ReadDir(fsys fs.FS, include, ignore *regexp.Regexp) (*ast.Package, *token.FileSet, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, kerrors.WithMsg(err, "Failed to read dir")
	}
	pkgName := ""
	astfiles := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, i := range entries {
		if i.IsDir() {
			continue
		}
		if !i.Type().IsRegular() {
			continue
		}
		filename := i.Name()
		if path.Ext(filename) != ".go" || strings.HasSuffix(filename, "_test.go") {
			continue
		}
		if include != nil && !include.MatchString(filename) {
			continue
		}
		if ignore != nil && ignore.MatchString(filename) {
			continue
		}
		astfile, err := parseGoFile(fset, fsys, filename)
		if err != nil {
			return nil, nil, err
		}
		if pkgName == "" {
			pkgName = astfile.Name.Name
		} else if astfile.Name.Name != pkgName {
			return nil, nil, kerrors.WithKind(nil, ErrorConflictingPackage{}, "Conflicting package names")
		}
		astfiles[filename] = astfile
	}
	return &ast.Package{
		Name:  pkgName,
		Files: astfiles,
	}, fset, nil
}

func parseGoFile(fset *token.FileSet, fsys fs.FS, filename string) (_ *ast.File, retErr error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return nil, kerrors.WithMsg(err, "Failed to open file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			retErr = errors.Join(retErr, kerrors.WithMsg(err, "Failed to close open file"))
		}
	}()
	astfile, err := parser.ParseFile(fset, filename, file, parser.ParseComments)
	if err != nil {
		return nil, kerrors.WithKind(err, ErrorParseFile{}, "Failed to parse file")
	}
	return astfile, nil
}

const (
	ObjKindUnknown    ObjKind = ""
	ObjKindGroupConst         = "group.const"
	ObjKindGroupVar           = "group.var"
	ObjKindGroupType          = "group.type"
	ObjKindDeclType           = "decl.type"
)

type (
	ObjKind string

	DirectiveInstance struct {
		Sigil     string
		Directive string
	}

	DirectiveObject struct {
		Directives []DirectiveInstance
		Kind       ObjKind
		Obj        ast.Node
	}

	pkgVisitor struct {
		sigils []string
		objs   []DirectiveObject
	}

	docCommentVisitor struct {
		sigils []string
		dirs   []DirectiveInstance
	}
)

func (v *docCommentVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.CommentGroup:
		return v
	case *ast.Comment:
		{
			text, ok := strings.CutPrefix(n.Text, "//")
			if !ok {
				return nil
			}
			for _, i := range v.sigils {
				if strings.HasPrefix(text, i) &&
					(len(text) == len(i) || text[len(i)] == ' ') {
					v.dirs = append(v.dirs, DirectiveInstance{
						Sigil:     i,
						Directive: strings.TrimSpace(strings.TrimPrefix(text, i)),
					})
					return nil
				}
			}
			return nil
		}
	default:
		return nil
	}
}

func (v *pkgVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	switch n := node.(type) {
	case *ast.File:
		return v
	case *ast.GenDecl:
		{
			kind := ObjKindUnknown
			switch n.Tok {
			case token.IMPORT:
				return nil
			case token.CONST:
				kind = ObjKindGroupConst
			case token.TYPE:
				kind = ObjKindGroupType
			case token.VAR:
				kind = ObjKindGroupVar
			}
			if kind != ObjKindUnknown && n.Doc != nil {
				visitor := &docCommentVisitor{
					sigils: v.sigils,
				}
				ast.Walk(visitor, n.Doc)
				if dirs := visitor.dirs; len(dirs) > 0 {
					v.objs = append(v.objs, DirectiveObject{
						Directives: dirs,
						Kind:       kind,
						Obj:        n,
					})
				}
			}
			return v
		}
	case *ast.TypeSpec:
		{
			if n.Doc == nil {
				return nil
			}
			visitor := &docCommentVisitor{
				sigils: v.sigils,
			}
			ast.Walk(visitor, n.Doc)
			if dirs := visitor.dirs; len(dirs) > 0 {
				v.objs = append(v.objs, DirectiveObject{
					Directives: dirs,
					Kind:       ObjKindDeclType,
					Obj:        n,
				})
			}
			return nil
		}
	default:
		return nil
	}
}

func FindDirectives(pkg *ast.Package, sigils []string) []DirectiveObject {
	visitor := &pkgVisitor{
		sigils: sigils,
	}
	filenames := make([]string, 0, len(pkg.Files))
	for k := range pkg.Files {
		filenames = append(filenames, k)
	}
	slices.Sort(filenames)
	for _, i := range filenames {
		ast.Walk(visitor, pkg.Files[i])
	}
	return visitor.objs
}
