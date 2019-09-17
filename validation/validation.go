package validation

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"
	"text/template"
)

const (
	validateTagName = "valid"
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
		Fields      []ValidationField
	}
)

func Execute(verbose bool, version, generatedFilepath, prefix, prefixValid, prefixHas string, validationIdents []string) {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		log.Fatal("Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		log.Fatal("Environment variable GOPACKAGE not provided by go generate")
	}

	fmt.Println(strings.Join([]string{
		"Generating validation",
		fmt.Sprintf("Package: %s", gopackage),
		fmt.Sprintf("Source file: %s", gofile),
		fmt.Sprintf("Validation structs: %s", strings.Join(validationIdents, ", ")),
	}, "; "))

	validations := parseDefinitions(gofile, validationIdents)

	tplmain, err := template.New("main").Parse(templateMain)
	if err != nil {
		log.Fatal(err)
	}

	tplvalidate, err := template.New("validate").Parse(templateValidate)
	if err != nil {
		log.Fatal(err)
	}

	genfile, err := os.OpenFile(generatedFilepath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer genfile.Close()
	genFileWriter := bufio.NewWriter(genfile)

	tplData := MainTemplateData{
		Generator: "go generate forge validation",
		Version:   version,
		Package:   gopackage,
	}
	if err := tplmain.Execute(genFileWriter, tplData); err != nil {
		log.Fatal(err)
	}

	for _, i := range validations {
		if verbose {
			fmt.Println("Detected validation " + i.Ident + " fields:")
			for _, i := range i.Fields {
				fmt.Printf("- %s %s %s\n", i.Ident, i.GoType, i.Key)
			}
		}
		tplData := ValidationTemplateData{
			Prefix:      prefix,
			Ident:       i.Ident,
			PrefixValid: prefixValid,
			PrefixHas:   prefixHas,
			Fields:      i.Fields,
		}
		if err := tplvalidate.Execute(genFileWriter, tplData); err != nil {
			log.Fatal(err)
		}
	}

	genFileWriter.Flush()

	fmt.Printf("Generated file: %s\n", generatedFilepath)
}

func parseDefinitions(gofile string, validationIdents []string) []ValidationDef {
	fset := token.NewFileSet()
	root, err := parser.ParseFile(fset, gofile, nil, parser.AllErrors)
	if err != nil {
		log.Fatal(err)
	}
	if root.Decls == nil {
		log.Fatal("No top level declarations")
	}

	validationDefs := []ValidationDef{}
	for _, ident := range validationIdents {
		fields := parseValidationFields(findFields(validateTagName, findStruct(ident, root.Decls), fset))
		validationDefs = append(validationDefs, ValidationDef{
			Ident:  ident,
			Fields: fields,
		})
	}

	return validationDefs
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

	log.Fatal(ident + " struct not found")
	return nil
}

func findFields(tagName string, validationDef *ast.StructType, fset *token.FileSet) []ASTField {
	fields := []ASTField{}
	for _, field := range validationDef.Fields.List {
		if field.Tag == nil {
			continue
		}
		structTag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		tagVal, ok := structTag.Lookup(tagName)
		if !ok {
			continue
		}

		goType := bytes.Buffer{}
		if err := printer.Fprint(&goType, fset, field.Type); err != nil {
			log.Fatal(err)
		}

		if len(field.Names) != 1 {
			log.Fatal("Only one field allowed per tag")
		}

		m := ASTField{
			Ident:  field.Names[0].Name,
			GoType: goType.String(),
			Tags:   tagVal,
		}
		fields = append(fields, m)
	}
	return fields
}

func parseValidationFields(astfields []ASTField) []ValidationField {
	if len(astfields) == 0 {
		log.Fatal("Validation struct does not contain a validated field")
	}

	fields := []ValidationField{}

	for _, i := range astfields {
		props := strings.SplitN(i.Tags, ",", 2)
		if len(props) < 1 {
			log.Fatal("Field tag must be fieldname[,flag]")
		}
		fieldname := strings.Title(props[0])
		f := ValidationField{
			Ident:  i.Ident,
			GoType: i.GoType,
			Key:    fieldname,
			Has:    false,
		}
		if len(props) > 1 {
			tags := strings.Split(props[1], ",")
			tagflag := parseFlag(tags[0])
			switch tagflag {
			case flagHas:
				if len(tags) != 1 {
					log.Fatal("Field tag must be fieldname,flag for field " + i.Ident)
				}
				f.Has = true
			default:
				if len(tags) != 1 {
					log.Fatal("Field tag must be fieldname,flag for field " + i.Ident)
				}
			}
		}
		fields = append(fields, f)
	}

	return fields
}

const (
	flagHas = iota
)

func parseFlag(flag string) int {
	switch flag {
	case "has":
		return flagHas
	default:
		log.Fatal("Illegal flag " + flag)
	}
	return -1
}
