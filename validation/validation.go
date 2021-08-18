package validation

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"log"
	"os"
	"reflect"
	"strings"
	"text/template"

	"xorkevin.dev/forge/writefs"
)

const (
	generatedFileMode = 0644
	generatedFileFlag = os.O_WRONLY | os.O_TRUNC | os.O_CREATE
)

var (
	// ErrEnv is returned when model is run outside of go generate
	ErrEnv = errors.New("Invalid execution environment")
	// ErrInvalidFile is returned when parsing an invalid model file
	ErrInvalidFile = errors.New("Invalid model file")
	// ErrInvalidValidator is returned when parsing a validator with invalid syntax
	ErrInvalidValidator = errors.New("Invalid validator")
)

type (
	ASTField struct {
		Ident  string
		GoType string
		Tags   string
	}

	ValidationDef struct {
		Ident  string
		Fields []ValidationField
	}

	ValidationField struct {
		Ident  string
		GoType string
		Key    string
		Has    bool
		Opt    bool
	}

	MainTemplateData struct {
		Generator string
		Version   string
		Package   string
	}

	ValidationTemplateData struct {
		Prefix      string
		Ident       string
		PrefixValid string
		PrefixHas   string
		PrefixOpt   string
		Fields      []ValidationField
	}
)

type (
	Opts struct {
		Verbose          bool
		Version          string
		Output           string
		Prefix           string
		PrefixValid      string
		PrefixHas        string
		PrefixOpt        string
		ValidationIdents []string
		Tag              string
	}

	execEnv struct {
		GoPackage string
		GoFile    string
	}
)

// Execute runs forge validation generation
func Execute(opts Opts) error {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		return fmt.Errorf("%w: Environment variable GOPACKAGE not provided by go generate", ErrEnv)
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		return fmt.Errorf("%w: Environment variable GOFILE not provided by go generate", ErrEnv)
	}

	return Generate(writefs.NewOS("."), os.DirFS("."), opts, execEnv{
		GoPackage: gopackage,
		GoFile:    gofile,
	})
}

func Generate(outputfs writefs.FS, inputfs fs.FS, opts Opts, env execEnv) error {
	fmt.Println(strings.Join([]string{
		"Generating validation",
		fmt.Sprintf("Package: %s", env.GoPackage),
		fmt.Sprintf("Source file: %s", env.GoFile),
		fmt.Sprintf("Validation structs: %s", strings.Join(opts.ValidationIdents, ", ")),
	}, "; "))

	validations, err := parseDefinitions(inputfs, env.GoFile, opts.ValidationIdents, opts.Tag)
	if err != nil {
		return fmt.Errorf("Failed to parse validation definitions: %w", err)
	}

	tplmain, err := template.New("main").Parse(templateMain)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateMain: %w", err)
	}

	tplvalidate, err := template.New("validate").Parse(templateValidate)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateValidate: %w", err)
	}

	file, err := outputfs.OpenFile(opts.Output, generatedFileFlag, generatedFileMode)
	if err != nil {
		return fmt.Errorf("Failed to write file %s: %w", opts.Output, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close open file %s: %v", opts.Output, err)
		}
	}()
	fwriter := bufio.NewWriter(file)

	tplData := MainTemplateData{
		Generator: "go generate forge validation",
		Version:   opts.Version,
		Package:   env.GoPackage,
	}
	if err := tplmain.Execute(fwriter, tplData); err != nil {
		return fmt.Errorf("Failed to execute main validation template: %w", err)
	}

	for _, i := range validations {
		if opts.Verbose {
			fmt.Println("Detected validation " + i.Ident + " fields:")
			for _, i := range i.Fields {
				fmt.Printf("- %s %s %s\n", i.Ident, i.GoType, i.Key)
			}
		}
		tplData := ValidationTemplateData{
			Prefix:      opts.Prefix,
			Ident:       i.Ident,
			PrefixValid: opts.PrefixValid,
			PrefixHas:   opts.PrefixHas,
			PrefixOpt:   opts.PrefixOpt,
			Fields:      i.Fields,
		}
		if err := tplvalidate.Execute(fwriter, tplData); err != nil {
			return fmt.Errorf("Failed to execute validation template for struct %s: %w", tplData.Ident, err)
		}
	}

	if err := fwriter.Flush(); err != nil {
		return fmt.Errorf("Failed to write to file %s: %w", opts.Output, err)
	}

	fmt.Printf("Generated file: %s\n", opts.Output)
	return nil
}

func parseDefinitions(inputfs fs.FS, gofile string, validationIdents []string, validateTag string) ([]ValidationDef, error) {
	fset := token.NewFileSet()
	var root *ast.File
	if err := func() error {
		file, err := inputfs.Open(gofile)
		if err != nil {
			return fmt.Errorf("Failed reading file %s: %w", gofile, err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("Failed to close open file %s: %v", gofile, err)
			}
		}()
		root, err = parser.ParseFile(fset, "", file, parser.AllErrors)
		if err != nil {
			return fmt.Errorf("Failed to parse file %s: %w", gofile, err)
		}
		return nil
	}(); err != nil {
		return nil, err
	}
	if root.Decls == nil {
		return nil, fmt.Errorf("%w: No top level declarations in %s", ErrInvalidFile, gofile)
	}

	validationDefs := []ValidationDef{}
	for _, ident := range validationIdents {
		validationStruct := findStruct(ident, root.Decls)
		if validationStruct == nil {
			return nil, fmt.Errorf("%w: Struct %s not found in %s", ErrInvalidFile, ident, gofile)
		}
		astFields, err := findFields(validateTag, validationStruct, fset)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse validation struct %s: %w", ident, err)
		}
		fields, err := parseValidationFields(astFields)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse validation fields for struct %s: %w", ident, err)
		}
		validationDefs = append(validationDefs, ValidationDef{
			Ident:  ident,
			Fields: fields,
		})
	}

	return validationDefs, nil
}

func findStruct(ident string, decls []ast.Decl) *ast.StructType {
	for _, i := range decls {
		typeDecl, ok := i.(*ast.GenDecl)
		if !ok || typeDecl.Tok != token.TYPE {
			continue
		}
		for _, j := range typeDecl.Specs {
			typeSpec, ok := j.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok || structType.Incomplete {
				continue
			}
			if typeSpec.Name.Name == ident {
				return structType
			}
		}
	}
	return nil
}

func findFields(tagName string, modelDef *ast.StructType, fset *token.FileSet) ([]ASTField, error) {
	fields := []ASTField{}
	for _, field := range modelDef.Fields.List {
		if field.Tag == nil {
			continue
		}
		structTag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		tagVal, ok := structTag.Lookup(tagName)
		if !ok {
			continue
		}

		if len(field.Names) != 1 {
			return nil, fmt.Errorf("%w: Only one field allowed per tag", ErrInvalidValidator)
		}

		ident := field.Names[0].Name

		goType := bytes.Buffer{}
		if err := printer.Fprint(&goType, fset, field.Type); err != nil {
			return nil, fmt.Errorf("Failed to print go struct field type for field %s: %w", ident, err)
		}

		m := ASTField{
			Ident:  ident,
			GoType: goType.String(),
			Tags:   tagVal,
		}
		fields = append(fields, m)
	}
	return fields, nil
}

func parseValidationFields(astfields []ASTField) ([]ValidationField, error) {
	if len(astfields) == 0 {
		return nil, fmt.Errorf("%w: Validation struct does not contain any validated fields", ErrInvalidValidator)
	}

	fields := []ValidationField{}

	for _, i := range astfields {
		props := strings.SplitN(i.Tags, ",", 2)
		if len(props[0]) == 0 {
			return nil, fmt.Errorf("%w: Field tag must be fieldname[,flag] for field %s", ErrInvalidValidator, i.Ident)
		}
		fieldname := strings.Title(props[0])
		f := ValidationField{
			Ident:  i.Ident,
			GoType: i.GoType,
			Key:    fieldname,
			Has:    false,
			Opt:    false,
		}
		if len(props) > 1 {
			tags := strings.Split(props[1], ",")
			tagflag, err := parseFlag(tags[0])
			if err != nil {
				return nil, fmt.Errorf("Failed to parse flags for field %s: %w", f.Ident, err)
			}
			switch tagflag {
			case flagHas:
				if len(tags) != 1 {
					return nil, fmt.Errorf("%w: Field tag must be fieldname,flag for field %s", ErrInvalidValidator, f.Ident)
				}
				f.Has = true
			case flagOpt:
				if len(tags) != 1 {
					return nil, fmt.Errorf("%w: Field tag must be fieldname,flag for field %s", ErrInvalidValidator, f.Ident)
				}
				f.Opt = true
			default:
				if len(tags) != 1 {
					return nil, fmt.Errorf("%w: Field tag must be fieldname,flag for field %s", ErrInvalidValidator, f.Ident)
				}
			}
		}
		fields = append(fields, f)
	}

	return fields, nil
}

const (
	flagHas = iota
	flagOpt
)

func parseFlag(flag string) (int, error) {
	switch flag {
	case "has":
		return flagHas, nil
	case "opt":
		return flagOpt, nil
	default:
		return 0, fmt.Errorf("%w: Illegal flag %s", ErrInvalidValidator, flag)
	}
}
