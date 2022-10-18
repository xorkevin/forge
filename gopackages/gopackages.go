package gopackages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"path"
	"regexp"
	"strings"

	"xorkevin.dev/kerrors"
)

func ReadDir(fsys fs.FS, include, ignore *regexp.Regexp) ([]*ast.File, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, kerrors.WithMsg(err, "Failed to read dir")
	}
	astfiles := make([]*ast.File, 0, len(entries))
	fset := token.NewFileSet()
	for _, i := range entries {
		if i.IsDir() {
			continue
		}
		if !i.Type().IsRegular() {
			continue
		}
		filename := i.Name()
		if path.Ext(filename) != ".go" {
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
			return nil, err
		}
		astfiles = append(astfiles, astfile)
	}
	return astfiles, nil
}

func parseGoFile(fset *token.FileSet, fsys fs.FS, filename string) (*ast.File, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return nil, kerrors.WithMsg(err, "Failed to open file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Println(kerrors.WithMsg(err, "Failed to close open file"))
		}
	}()
	astfile, err := parser.ParseFile(fset, filename, file, parser.ParseComments)
	if err != nil {
		return nil, kerrors.WithMsg(err, "Failed to parse file")
	}
	return astfile, nil
}

var (
	topLevelASTObjKinds = map[ast.ObjKind]struct{}{
		ast.Con: {},
		ast.Typ: {},
		ast.Var: {},
		ast.Fun: {},
	}
)

func FindDirectives(astfiles []*ast.File, sigils []string) map[string][]*ast.Object {
	dirToNode := map[string][]*ast.Object{}
	for _, i := range astfiles {
	objloop:
		for _, j := range i.Scope.Objects {
			if _, ok := topLevelASTObjKinds[j.Kind]; !ok {
				continue
			}
			var comments *ast.CommentGroup
			switch k := j.Decl.(type) {
			// Field, XxxSpec, FuncDecl, LabeledStmt, AssignStmt, Scope
			case *ast.ValueSpec:
				comments = k.Doc
			case *ast.TypeSpec:
				comments = k.Doc
			case *ast.FuncDecl:
				comments = k.Doc
			}
			if comments == nil {
				continue
			}
			for _, c := range comments.List {
				for _, s := range sigils {
					if strings.HasPrefix(c.Text, s) {
						dirToNode[s] = append(dirToNode[s], j)
						continue objloop
					}
				}
			}
		}
	}
	return dirToNode
}
