package validation

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"xorkevin.dev/forge/gopackages"
	"xorkevin.dev/kerrors"
	"xorkevin.dev/kfs"
	"xorkevin.dev/klog"
)

const (
	generatedFileMode = 0o644
	generatedFileFlag = os.O_WRONLY | os.O_TRUNC | os.O_CREATE
)

var (
	// ErrEnv is returned when validation is run outside of go generate
	ErrEnv errEnv
	// ErrInvalidFile is returned when parsing an invalid validation file
	ErrInvalidFile errInvalidFile
	// ErrInvalidValidator is returned when parsing a validator with invalid syntax
	ErrInvalidValidator errInvalidValidator
)

type (
	errEnv              struct{}
	errInvalidFile      struct{}
	errInvalidValidator struct{}
)

func (e errEnv) Error() string {
	return "Invalid execution environment"
}

func (e errInvalidFile) Error() string {
	return "Invalid file"
}

func (e errInvalidValidator) Error() string {
	return "Invalid validator"
}

type (
	astField struct {
		Ident string
		Tags  string
	}

	validationDef struct {
		Ident  string
		Fields []validationField
	}

	validationField struct {
		Ident string
		Key   string
		Has   bool
		Opt   bool
	}

	mainTemplateData struct {
		Generator string
		Version   string
		Package   string
	}

	validationTemplateData struct {
		Prefix      string
		Ident       string
		PrefixValid string
		PrefixHas   string
		PrefixOpt   string
		Fields      []validationField
	}
)

type (
	Opts struct {
		Output      string
		Prefix      string
		PrefixValid string
		PrefixHas   string
		PrefixOpt   string
		Include     string
		Ignore      string
		Directive   string
		Tag         string
	}

	ExecEnv struct {
		GoPackage string
	}
)

// Execute runs forge validation generation
func Execute(log klog.Logger, version string, opts Opts) error {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		return kerrors.WithKind(nil, ErrEnv, "Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		return kerrors.WithKind(nil, ErrEnv, "Environment variable GOFILE not provided by go generate")
	}

	ctx := klog.CtxWithAttrs(context.Background(),
		klog.AString("package", gopackage),
		klog.AString("source", gofile),
	)

	return Generate(ctx, log, kfs.DirFS("."), os.DirFS("."), version, opts, ExecEnv{
		GoPackage: gopackage,
	})
}

func Generate(ctx context.Context, log klog.Logger, outputfs fs.FS, inputfs fs.FS, version string, opts Opts, env ExecEnv) (retErr error) {
	l := klog.NewLevelLogger(log)

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

	astpkg, _, err := gopackages.ReadDir(inputfs, includePattern, ignorePattern)
	if err != nil {
		return err
	}
	if astpkg.Name != env.GoPackage {
		return kerrors.WithKind(nil, ErrEnv, "Environment variable GOPACKAGE does not match directory package")
	}

	directiveObjects := gopackages.FindDirectives(astpkg, []string{opts.Directive})
	if len(directiveObjects) == 0 {
		return kerrors.WithKind(nil, ErrInvalidFile, "No validations found")
	}

	validations, err := parseDefinitions(directiveObjects, opts.Tag)
	if err != nil {
		return err
	}

	tplmain, err := template.New("main").Parse(templateMain)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateMain")
	}

	tplvalidate, err := template.New("validate").Parse(templateValidate)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateValidate")
	}

	file, err := kfs.OpenFile(outputfs, opts.Output, generatedFileFlag, generatedFileMode)
	if err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to write file %s", opts.Output))
	}
	defer func() {
		if err := file.Close(); err != nil {
			retErr = errors.Join(retErr, kerrors.WithMsg(err, fmt.Sprintf("Failed to close open file %s", opts.Output)))
		}
	}()
	fwriter := bufio.NewWriter(file)

	tplData := mainTemplateData{
		Generator: "go generate forge validation",
		Version:   version,
		Package:   env.GoPackage,
	}
	if err := tplmain.Execute(fwriter, tplData); err != nil {
		return kerrors.WithMsg(err, "Failed to execute main validation template")
	}

	for _, i := range validations {
		vctx := klog.CtxWithAttrs(ctx, klog.AString("validation", i.Ident))
		l.Debug(vctx, "Detected validation", klog.AAny("fields", i.Fields))

		tplData := validationTemplateData{
			Prefix:      opts.Prefix,
			Ident:       i.Ident,
			PrefixValid: opts.PrefixValid,
			PrefixHas:   opts.PrefixHas,
			PrefixOpt:   opts.PrefixOpt,
			Fields:      i.Fields,
		}
		if err := tplvalidate.Execute(fwriter, tplData); err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to execute validation template for struct: %s", tplData.Ident))
		}
	}

	if err := fwriter.Flush(); err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to write to file: %s", opts.Output))
	}

	l.Info(ctx, "Generated validation file", klog.AString("output", opts.Output))
	return nil
}

func parseDefinitions(directiveObjects []gopackages.DirectiveObject, validateTag string) ([]validationDef, error) {
	var validationDefs []validationDef
	for _, i := range directiveObjects {
		if i.Kind != gopackages.ObjKindDeclType {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Validation directive used on non-type declaration")
		}
		typeSpec, ok := i.Obj.(*ast.TypeSpec)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Unexpected directive object type")
		}
		structName := typeSpec.Name.Name
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Validation directive used on non-struct type declaration")
		}
		if structType.Incomplete {
			return nil, kerrors.WithMsg(nil, "Unexpected incomplete struct definition")
		}
		astFields, err := findFields(validateTag, structType)
		if err != nil {
			return nil, err
		}
		if len(astFields) == 0 {
			return nil, kerrors.WithKind(nil, ErrInvalidValidator, "No field validations found on struct")
		}
		fields, err := parseValidationFields(astFields)
		if err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse validation fields for struct %s", structName))
		}
		validationDefs = append(validationDefs, validationDef{
			Ident:  structName,
			Fields: fields,
		})
	}

	return validationDefs, nil
}

func findFields(tagName string, structType *ast.StructType) ([]astField, error) {
	var fields []astField
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
			return nil, kerrors.WithKind(nil, ErrInvalidValidator, "Only one field allowed per tag")
		}

		m := astField{
			Ident: field.Names[0].Name,
			Tags:  tagVal,
		}
		fields = append(fields, m)
	}
	return fields, nil
}

func parseValidationFields(astFields []astField) ([]validationField, error) {
	fields := make([]validationField, 0, len(astFields))

	for _, i := range astFields {
		fieldname, tag, _ := strings.Cut(i.Tags, ",")
		if fieldname == "" {
			return nil, kerrors.WithKind(nil, ErrInvalidValidator, fmt.Sprintf("Field tag must be fieldname[,flag] for field %s", i.Ident))
		}
		f := validationField{
			Ident: i.Ident,
			Key:   strings.ToUpper(fieldname[:1]) + fieldname[1:],
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
		return flagUnknown, kerrors.WithKind(nil, ErrInvalidValidator, fmt.Sprintf("Illegal flag %s", flag))
	}
}
