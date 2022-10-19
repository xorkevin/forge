package validation

import (
	"bufio"
	"fmt"
	"go/ast"
	"io/fs"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"xorkevin.dev/forge/gopackages"
	"xorkevin.dev/forge/writefs"
	"xorkevin.dev/kerrors"
)

const (
	generatedFileMode = 0644
	generatedFileFlag = os.O_WRONLY | os.O_TRUNC | os.O_CREATE
)

type (
	// ErrorEnv is returned when validation is run outside of go generate
	ErrorEnv struct{}
	// ErrorInvalidFile is returned when parsing an invalid validation file
	ErrorInvalidFile struct{}
	// ErrorInvalidValidator is returned when parsing a validator with invalid syntax
	ErrorInvalidValidator struct{}
)

func (e ErrorEnv) Error() string {
	return "Invalid execution environment"
}
func (e ErrorInvalidFile) Error() string {
	return "Invalid file"
}
func (e ErrorInvalidValidator) Error() string {
	return "Invalid validator"
}

type (
	ASTField struct {
		Ident string
		Tags  string
	}

	ValidationDef struct {
		Ident  string
		Fields []ValidationField
	}

	ValidationField struct {
		Ident string
		Key   string
		Has   bool
		Opt   bool
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
		Verbose     bool
		Version     string
		Output      string
		Prefix      string
		Include     string
		Ignore      string
		Directive   string
		PrefixValid string
		PrefixHas   string
		PrefixOpt   string
		Tag         string
	}

	execEnv struct {
		GoPackage string
	}
)

// Execute runs forge validation generation
func Execute(opts Opts) error {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		return kerrors.WithKind(nil, ErrorEnv{}, "Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		return kerrors.WithKind(nil, ErrorEnv{}, "Environment variable GOFILE not provided by go generate")
	}

	fmt.Println(strings.Join([]string{
		"Generating validation",
		fmt.Sprintf("Package: %s", gopackage),
		fmt.Sprintf("Source file: %s", gofile),
	}, "; "))

	return Generate(writefs.NewOS("."), os.DirFS("."), opts, execEnv{
		GoPackage: gopackage,
	})
}

func Generate(outputfs writefs.FS, inputfs fs.FS, opts Opts, env execEnv) error {
	var includePattern, ignorePattern *regexp.Regexp
	if opts.Include != "" {
		var err error
		includePattern, err = regexp.Compile(opts.Include)
		if err != nil {
			return kerrors.WithMsg(err, "Invalid include regex")
		}
	}
	if opts.Ignore != "" {
		var err error
		ignorePattern, err = regexp.Compile(opts.Ignore)
		if err != nil {
			return kerrors.WithMsg(err, "Invalid ignore regex")
		}
	}

	astpkg, err := gopackages.ReadDir(inputfs, includePattern, ignorePattern)
	if err != nil {
		return err
	}
	directiveObjects := gopackages.FindDirectives(astpkg, []string{opts.Directive})
	if len(directiveObjects) == 0 {
		return kerrors.WithKind(nil, ErrorInvalidFile{}, "No validations found")
	}

	validations, err := parseDefinitions(directiveObjects, opts.Tag)
	if err != nil {
		return err
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
				fmt.Printf("- %s %s\n", i.Ident, i.Key)
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

func parseDefinitions(directiveObjects []gopackages.DirectiveObject, validateTag string) ([]ValidationDef, error) {
	var validationDefs []ValidationDef
	for _, i := range directiveObjects {
		if i.Kind != gopackages.ObjKindDeclType {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Validation directive used on non-type declaration")
		}
		typeSpec, ok := i.Obj.(*ast.TypeSpec)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Unexpected directive object type")
		}
		structName := typeSpec.Name.Name
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Validation directive used on non-struct type declaration")
		}
		if structType.Incomplete {
			return nil, kerrors.WithMsg(nil, "Unexpected incomplete struct definition")
		}
		astFields, err := findFields(validateTag, structType)
		if err != nil {
			return nil, err
		}
		if len(astFields) == 0 {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "No field validations found on struct")
		}
		fields, err := parseValidationFields(astFields)
		if err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse validation fields for struct %s", structName))
		}
		validationDefs = append(validationDefs, ValidationDef{
			Ident:  structName,
			Fields: fields,
		})
	}

	return validationDefs, nil
}

func findFields(tagName string, structType *ast.StructType) ([]ASTField, error) {
	var fields []ASTField
	for _, field := range structType.Fields.List {
		if field.Tag == nil {
			continue
		}
		structTags := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		tagVal, ok := structTags.Lookup(tagName)
		if !ok {
			continue
		}

		if len(field.Names) != 1 {
			return nil, kerrors.WithKind(nil, ErrorInvalidValidator{}, "Only one field allowed per tag")
		}

		m := ASTField{
			Ident: field.Names[0].Name,
			Tags:  tagVal,
		}
		fields = append(fields, m)
	}
	return fields, nil
}

func parseValidationFields(astFields []ASTField) ([]ValidationField, error) {
	fields := make([]ValidationField, 0, len(astFields))

	for _, i := range astFields {
		fieldname, tag, _ := strings.Cut(i.Tags, ",")
		if fieldname == "" {
			return nil, kerrors.WithKind(nil, ErrorInvalidValidator{}, fmt.Sprintf("Field tag must be fieldname[,flag] for field %s", i.Ident))
		}
		f := ValidationField{
			Ident: i.Ident,
			Key:   strings.Title(fieldname),
			Has:   false,
			Opt:   false,
		}
		if tag != "" {
			tagflag, err := parseFlag(tag)
			if err != nil {
				return nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse flags for field %s", f.Ident))
			}
			switch tagflag {
			case flagHas:
				f.Has = true
			case flagOpt:
				f.Opt = true
			}
		}
		fields = append(fields, f)
	}

	return fields, nil
}

const (
	flagUnknown = iota
	flagHas
	flagOpt
)

func parseFlag(flag string) (int, error) {
	switch flag {
	case "has":
		return flagHas, nil
	case "opt":
		return flagOpt, nil
	default:
		return flagUnknown, kerrors.WithKind(nil, ErrorInvalidValidator{}, fmt.Sprintf("Illegal flag %s", flag))
	}
}
