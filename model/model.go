package model

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
	// ErrInvalidModel is returned when parsing a model with invalid syntax
	ErrInvalidModel = errors.New("Invalid model")
)

type (
	ASTField struct {
		Ident  string
		GoType string
		Tags   string
	}

	ModelDef struct {
		Ident    string
		Fields   []ModelField
		Indicies [][]ModelField
	}

	QueryDef struct {
		Ident       string
		Fields      []QueryField
		QueryFields []QueryField
	}

	ModelField struct {
		Ident  string
		GoType string
		DBName string
		DBType string
		Num    int
	}

	QueryField struct {
		Ident  string
		GoType string
		DBName string
		DBType string
		Num    int
		Mode   QueryFlag
		Cond   []CondField
	}

	CondField struct {
		Kind  CondType
		Field ModelField
	}

	ModelIndex struct {
		Name    string
		Columns string
	}

	ModelSQLStrings struct {
		Setup            string
		DBNames          string
		Placeholders     string
		PlaceholderTpl   string
		PlaceholderCount string
		Idents           string
		IdentRefs        string
		Indicies         []ModelIndex
		ColNum           string
	}

	ModelTemplateData struct {
		Generator  string
		Version    string
		Package    string
		Prefix     string
		Imports    string
		ModelIdent string
		SQL        ModelSQLStrings
	}

	QuerySQLStrings struct {
		DBNames      string
		Placeholders string
		Idents       string
		IdentRefs    string
		ColNum       string
	}

	QueryCondSQLStrings struct {
		DBCond          string
		IdentParams     string
		IdentArgs       string
		ArrIdentArgs    []string
		ArrIdentArgsLen string
		IdentNames      string
		ParamCount      int
	}

	QueryTemplateData struct {
		Prefix       string
		ModelIdent   string
		PrimaryField QueryField
		SQL          QuerySQLStrings
		SQLCond      QueryCondSQLStrings
	}
)

type (
	Opts struct {
		Verbose     bool
		Version     string
		Output      string
		Prefix      string
		ModelIdent  string
		QueryIdents []string
		ModelTag    string
		QueryTag    string
	}

	execEnv struct {
		GoPackage string
		GoFile    string
	}
)

// Execute runs forge model generation
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
		"Generating model",
		fmt.Sprintf("Package: %s", env.GoPackage),
		fmt.Sprintf("Source file: %s", env.GoFile),
		fmt.Sprintf("Model ident: %s", opts.ModelIdent),
		fmt.Sprintf("Additional queries: %s", strings.Join(opts.QueryIdents, ", ")),
	}, "; "))

	modelDef, queryDefs, err := parseDefinitions(inputfs, env.GoFile, opts.ModelIdent, opts.QueryIdents, opts.ModelTag, opts.QueryTag)
	if err != nil {
		return fmt.Errorf("Failed to parse model and query definitions: %w", err)
	}

	tplmodel, err := template.New("model").Parse(templateModel)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateModel: %w", err)
	}
	tplgetoneeq, err := template.New("getoneeq").Parse(templateGetOneEq)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateGetOneEq: %w", err)
	}
	tplgetgroup, err := template.New("getgroup").Parse(templateGetGroup)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateGetGroup: %w", err)
	}
	tplgetgroupeq, err := template.New("getgroupeq").Parse(templateGetGroupEq)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateGetGroupEq: %w", err)
	}
	tplupdeq, err := template.New("updeq").Parse(templateUpdEq)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateUpdEq: %w", err)
	}
	tpldeleq, err := template.New("deleq").Parse(templateDelEq)
	if err != nil {
		return fmt.Errorf("Failed to parse template templateDelEq: %w", err)
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

	tplData := ModelTemplateData{
		Generator:  "go generate forge model",
		Version:    opts.Version,
		Package:    env.GoPackage,
		Prefix:     opts.Prefix,
		ModelIdent: modelDef.Ident,
		SQL:        modelDef.genModelSQL(),
	}
	if err := tplmodel.Execute(fwriter, tplData); err != nil {
		return fmt.Errorf("Failed to execute model template for struct %s: %w", modelDef.Ident, err)
	}

	if opts.Verbose {
		fmt.Println("Detected model fields:")
		for _, i := range modelDef.Fields {
			fmt.Printf("- %s %s\n", i.Ident, i.GoType)
		}
	}

	for _, queryDef := range queryDefs {
		if opts.Verbose {
			fmt.Println("Detected query " + queryDef.Ident + " fields:")
			for _, i := range queryDef.Fields {
				fmt.Printf("- %s %s\n", i.Ident, i.GoType)
			}
		}
		querySQLStrings := queryDef.genQuerySQL()
		for _, i := range queryDef.QueryFields {
			tplData := QueryTemplateData{
				Prefix:       opts.Prefix,
				ModelIdent:   queryDef.Ident,
				PrimaryField: i,
				SQL:          querySQLStrings,
			}
			switch i.Mode {
			case flagGetOneEq:
				tplData.SQLCond = i.genQueryCondSQL(0)
				if err := tplgetoneeq.Execute(fwriter, tplData); err != nil {
					return fmt.Errorf("Failed to execute getoneeq template for field %s on struct %s: %w", tplData.PrimaryField.Ident, tplData.ModelIdent, err)
				}
			case flagGetGroup:
				if err := tplgetgroup.Execute(fwriter, tplData); err != nil {
					return fmt.Errorf("Failed to execute getgroup template for field %s on struct %s: %w", tplData.PrimaryField.Ident, tplData.ModelIdent, err)
				}
			case flagGetGroupEq:
				tplData.SQLCond = i.genQueryCondSQL(2)
				if err := tplgetgroupeq.Execute(fwriter, tplData); err != nil {
					return fmt.Errorf("Failed to execute getgroupeq template for field %s on struct %s: %w", tplData.PrimaryField.Ident, tplData.ModelIdent, err)
				}
			case flagUpdEq:
				tplData.SQLCond = i.genQueryCondSQL(len(queryDef.Fields))
				if err := tplupdeq.Execute(fwriter, tplData); err != nil {
					return fmt.Errorf("Failed to execute updeq template for field %s on struct %s: %w", tplData.PrimaryField.Ident, tplData.ModelIdent, err)
				}
			case flagDelEq:
				tplData.SQLCond = i.genQueryCondSQL(0)
				if err := tpldeleq.Execute(fwriter, tplData); err != nil {
					return fmt.Errorf("Failed to execute deleq template for field %s on struct %s: %w", tplData.PrimaryField.Ident, tplData.ModelIdent, err)
				}
			}
		}
	}

	if err := fwriter.Flush(); err != nil {
		return fmt.Errorf("Failed to write to file %s: %w", opts.Output, err)
	}

	fmt.Printf("Generated file: %s\n", opts.Output)
	return nil
}

func parseDefinitions(inputfs fs.FS, gofile string, modelIdent string, queryIdents []string, modelTag string, queryTag string) (*ModelDef, []QueryDef, error) {
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
		return nil, nil, err
	}
	if root.Decls == nil {
		return nil, nil, fmt.Errorf("%w: No top level declarations in %s", ErrInvalidFile, gofile)
	}

	modelStruct := findStruct(modelIdent, root.Decls)
	if modelStruct == nil {
		return nil, nil, fmt.Errorf("%w: Struct %s not found in %s", ErrInvalidFile, modelIdent, gofile)
	}
	modelASTFields, err := findFields(modelTag, modelStruct, fset)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse model struct %s: %w", modelIdent, err)
	}
	modelFields, seenFields, indicies, err := parseModelFields(modelASTFields)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse model fields for struct %s: %w", modelIdent, err)
	}

	queryDefs := []QueryDef{}
	for _, ident := range queryIdents {
		queryStruct := findStruct(ident, root.Decls)
		if queryStruct == nil {
			return nil, nil, fmt.Errorf("%w: Struct %s not found in %s", ErrInvalidFile, ident, gofile)
		}
		queryASTFields, err := findFields(queryTag, queryStruct, fset)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to parse query struct %s: %w", ident, err)
		}
		fields, queries, err := parseQueryFields(queryASTFields, seenFields)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to parse query fields for struct %s: %w", ident, err)
		}
		queryDefs = append(queryDefs, QueryDef{
			Ident:       ident,
			Fields:      fields,
			QueryFields: queries,
		})
	}

	return &ModelDef{
		Ident:    modelIdent,
		Fields:   modelFields,
		Indicies: indicies,
	}, queryDefs, nil
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
			return nil, fmt.Errorf("%w: Only one field allowed per tag", ErrInvalidModel)
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

func parseModelFields(astfields []ASTField) ([]ModelField, map[string]ModelField, [][]ModelField, error) {
	if len(astfields) == 0 {
		return nil, nil, nil, fmt.Errorf("%w: Model struct does not contain any model fields", ErrInvalidModel)
	}

	seenFields := map[string]ModelField{}
	tagIndicies := [][]string{}

	fields := []ModelField{}
	for n, i := range astfields {
		tags := strings.SplitN(i.Tags, ",", 2)
		if len(tags) < 2 {
			return nil, nil, nil, fmt.Errorf("%w: Model field tag must be dbname,dbtype[;opt[,fields ...][; ...]] for field %s", ErrInvalidModel, i.Ident)
		}
		dbName := tags[0]
		opts := strings.Split(tags[1], ";")
		dbType := opts[0]
		if len(dbName) == 0 {
			return nil, nil, nil, fmt.Errorf("%w: %s dbname not set", ErrInvalidModel, i.Ident)
		}
		if len(dbType) == 0 {
			return nil, nil, nil, fmt.Errorf("%w: %s dbtype not set", ErrInvalidModel, i.Ident)
		}
		if dup, ok := seenFields[dbName]; ok {
			return nil, nil, nil, fmt.Errorf("%w: Duplicate field %s on %s and %s", ErrInvalidModel, dbName, i.Ident, dup.Ident)
		}
		f := ModelField{
			Ident:  i.Ident,
			GoType: i.GoType,
			DBName: dbName,
			DBType: dbType,
			Num:    n + 1,
		}
		for _, i := range opts[1:] {
			tags := strings.Split(i, ",")
			opt, err := parseModelOpt(tags[0])
			if err != nil {
				return nil, nil, nil, fmt.Errorf("Failed to parse model opt for field %s: %w", f.Ident, err)
			}
			switch opt {
			case optIndex:
				tagIndicies = append(tagIndicies, append(tags[1:], dbName))
			}
		}
		seenFields[dbName] = f
		fields = append(fields, f)
	}

	indicies := [][]ModelField{}
	for _, i := range tagIndicies {
		k := make([]ModelField, 0, len(i))
		for _, j := range i {
			f, ok := seenFields[j]
			if !ok {
				return nil, nil, nil, fmt.Errorf("%w: No field %s for index", ErrInvalidModel, j)
			}
			k = append(k, f)
		}
		indicies = append(indicies, k)
	}

	return fields, seenFields, indicies, nil
}

func parseQueryFields(astfields []ASTField, seenFields map[string]ModelField) ([]QueryField, []QueryField, error) {
	hasQF := false
	queryFields := []QueryField{}

	fields := []QueryField{}
	for n, i := range astfields {
		props := strings.SplitN(i.Tags, ";", 2)
		if len(props) < 1 {
			return nil, nil, fmt.Errorf("%w: Query field tag must be dbname[;flag[,args ...][; ...]] for field %s", ErrInvalidModel, i.Ident)
		}
		dbName := props[0]
		modelField, ok := seenFields[dbName]
		if !ok || i.GoType != modelField.GoType {
			return nil, nil, fmt.Errorf("%w: Field %s with type %s does not exist on model", ErrInvalidModel, dbName, i.GoType)
		}
		f := QueryField{
			Ident:  i.Ident,
			GoType: i.GoType,
			DBName: dbName,
			DBType: modelField.DBType,
			Num:    n + 1,
		}
		fields = append(fields, f)
		if len(props) > 1 {
			hasQF = true
			for _, t := range strings.Split(props[1], ";") {
				tags := strings.Split(t, ",")
				tagflag, err := parseFlag(tags[0])
				if err != nil {
					return nil, nil, fmt.Errorf("Failed to parse flags for field %s: %w", f.Ident, err)
				}
				f.Mode = tagflag
				switch tagflag {
				case flagGetOneEq, flagGetGroupEq, flagUpdEq, flagDelEq:
					if len(tags) < 2 {
						return nil, nil, fmt.Errorf("%w: Query field tag must be dbname;flag,fields,... for tag %s on field %s", ErrInvalidModel, tags[0], f.Ident)
					}
					k := make([]CondField, 0, len(tags[1:]))
					for _, cond := range tags[1:] {
						condName, kind, err := parseCondField(cond)
						if err != nil {
							return nil, nil, fmt.Errorf("Failed to parse condition field for tag %s on field %s: %w", tags[0], f.Ident, err)
						}
						if field, ok := seenFields[condName]; ok {
							k = append(k, CondField{
								Kind:  kind,
								Field: field,
							})
						} else {
							return nil, nil, fmt.Errorf("%w: Invalid condition field %s for field %s", ErrInvalidModel, condName, i.Ident)
						}
					}
					f.Cond = k
				default:
					if len(tags) != 1 {
						return nil, nil, fmt.Errorf("%w: Field tag must be dbname,flag for tag %s on field %s", ErrInvalidModel, tags[0], i.Ident)
					}
				}
				queryFields = append(queryFields, f)
			}
		}
	}

	if !hasQF {
		return nil, nil, fmt.Errorf("%w: Query does not contain a query field", ErrInvalidModel)
	}

	return fields, queryFields, nil
}

type (
	ModelOpt int
)

const (
	optIndex ModelOpt = iota
)

func parseModelOpt(opt string) (ModelOpt, error) {
	switch opt {
	case "index":
		return optIndex, nil
	default:
		return 0, fmt.Errorf("%w: Illegal opt %s", ErrInvalidModel, opt)
	}
}

type (
	QueryFlag int
)

const (
	flagGetOneEq QueryFlag = iota
	flagGetGroup
	flagGetGroupEq
	flagGetGroupSet
	flagUpdEq
	flagUpdSet
	flagDelEq
	flagDelSet
)

func parseFlag(flag string) (QueryFlag, error) {
	switch flag {
	case "getoneeq":
		return flagGetOneEq, nil
	case "getgroup":
		return flagGetGroup, nil
	case "getgroupeq":
		return flagGetGroupEq, nil
	case "updeq":
		return flagUpdEq, nil
	case "deleq":
		return flagDelEq, nil
	default:
		return 0, fmt.Errorf("%w: Illegal flag %s", ErrInvalidModel, flag)
	}
}

type (
	CondType int
)

const (
	condEq CondType = iota
	condNeq
	condLt
	condLeq
	condGt
	condGeq
	condArr
	condLike
)

func parseCondField(field string) (string, CondType, error) {
	k := strings.SplitN(field, "|", 2)
	if len(k) == 2 {
		cond, err := parseCond(k[1])
		if err != nil {
			return "", 0, err
		}
		return k[0], cond, nil
	}
	return field, condEq, nil
}

func parseCond(cond string) (CondType, error) {
	switch cond {
	case "eq":
		return condEq, nil
	case "neq":
		return condNeq, nil
	case "lt":
		return condLt, nil
	case "leq":
		return condLeq, nil
	case "gt":
		return condGt, nil
	case "geq":
		return condGeq, nil
	case "arr":
		return condArr, nil
	case "like":
		return condLike, nil
	default:
		return 0, fmt.Errorf("%w: Illegal cond type %s", ErrInvalidModel, cond)
	}
}

func dbTypeIsArray(dbType string) bool {
	return strings.Contains(dbType, "ARRAY")
}

func (m *ModelDef) genModelSQL() ModelSQLStrings {
	colNum := len(m.Fields)
	sqlDefs := make([]string, 0, colNum)
	sqlDBNames := make([]string, 0, colNum)
	sqlPlaceholders := make([]string, 0, colNum)
	sqlPlaceholderTpl := make([]string, 0, colNum)
	sqlPlaceholderCount := make([]string, 0, colNum)
	sqlIdents := make([]string, 0, colNum)
	sqlIdentRefs := make([]string, 0, colNum)

	placeholderStart := 1
	for n, i := range m.Fields {
		sqlDefs = append(sqlDefs, fmt.Sprintf("%s %s", i.DBName, i.DBType))
		sqlDBNames = append(sqlDBNames, i.DBName)
		sqlPlaceholders = append(sqlPlaceholders, fmt.Sprintf("$%d", n+placeholderStart))
		sqlPlaceholderTpl = append(sqlPlaceholderTpl, "$%d")
		sqlPlaceholderCount = append(sqlPlaceholderCount, fmt.Sprintf("n+%d", n+placeholderStart))
		if dbTypeIsArray(i.DBType) {
			sqlIdents = append(sqlIdents, fmt.Sprintf("pq.Array(m.%s)", i.Ident))
			sqlIdentRefs = append(sqlIdentRefs, fmt.Sprintf("pq.Array(&m.%s)", i.Ident))
		} else {
			sqlIdents = append(sqlIdents, fmt.Sprintf("m.%s", i.Ident))
			sqlIdentRefs = append(sqlIdentRefs, fmt.Sprintf("&m.%s", i.Ident))
		}
	}

	sqlIndicies := make([]ModelIndex, 0, len(m.Indicies))
	for _, i := range m.Indicies {
		k := make([]string, 0, len(i))
		for _, j := range i {
			k = append(k, j.DBName)
		}
		sqlIndicies = append(sqlIndicies, ModelIndex{
			Name:    strings.Join(k, "__"),
			Columns: strings.Join(k, ", "),
		})
	}

	return ModelSQLStrings{
		Setup:            strings.Join(sqlDefs, ", "),
		DBNames:          strings.Join(sqlDBNames, ", "),
		Placeholders:     strings.Join(sqlPlaceholders, ", "),
		PlaceholderTpl:   strings.Join(sqlPlaceholderTpl, ", "),
		PlaceholderCount: strings.Join(sqlPlaceholderCount, ", "),
		Idents:           strings.Join(sqlIdents, ", "),
		IdentRefs:        strings.Join(sqlIdentRefs, ", "),
		Indicies:         sqlIndicies,
		ColNum:           fmt.Sprintf("%d", colNum),
	}
}

func (q *QueryDef) genQuerySQL() QuerySQLStrings {
	colNum := len(q.Fields)
	sqlDBNames := make([]string, 0, colNum)
	sqlPlaceholders := make([]string, 0, colNum)
	sqlIdents := make([]string, 0, colNum)
	sqlIdentRefs := make([]string, 0, colNum)

	placeholderStart := 1
	for n, i := range q.Fields {
		sqlDBNames = append(sqlDBNames, i.DBName)
		sqlPlaceholders = append(sqlPlaceholders, fmt.Sprintf("$%d", n+placeholderStart))
		if dbTypeIsArray(i.DBType) {
			sqlIdents = append(sqlIdents, fmt.Sprintf("pq.Array(m.%s)", i.Ident))
			sqlIdentRefs = append(sqlIdentRefs, fmt.Sprintf("pq.Array(&m.%s)", i.Ident))
		} else {
			sqlIdents = append(sqlIdents, fmt.Sprintf("m.%s", i.Ident))
			sqlIdentRefs = append(sqlIdentRefs, fmt.Sprintf("&m.%s", i.Ident))
		}
	}

	return QuerySQLStrings{
		DBNames:      strings.Join(sqlDBNames, ", "),
		Placeholders: strings.Join(sqlPlaceholders, ", "),
		Idents:       strings.Join(sqlIdents, ", "),
		IdentRefs:    strings.Join(sqlIdentRefs, ", "),
		ColNum:       fmt.Sprintf("%d", colNum),
	}
}

func (q *QueryField) genQueryCondSQL(offset int) QueryCondSQLStrings {
	sqlDBCond := make([]string, 0, len(q.Cond))
	sqlIdentParams := make([]string, 0, len(q.Cond))
	sqlIdentArgs := make([]string, 0, len(q.Cond))
	sqlArrIdentArgs := make([]string, 0, len(q.Cond))
	sqlArrIdentArgsLen := make([]string, 0, len(q.Cond))
	sqlIdentNames := make([]string, 0, len(q.Cond))
	paramCount := offset
	for _, i := range q.Cond {
		paramName := strings.ToLower(i.Field.Ident)
		dbName := i.Field.DBName
		paramType := i.Field.GoType
		identName := i.Field.Ident
		condText := "="
		switch i.Kind {
		case condNeq:
			identName = "Neq" + identName
			condText = "<>"
		case condLt:
			identName = "Lt" + identName
			condText = "<"
		case condLeq:
			identName = "Leq" + identName
			condText = "<="
		case condGt:
			identName = "Gt" + identName
			condText = ">"
		case condGeq:
			identName = "Geq" + identName
			condText = ">="
		case condArr:
			paramType = "[]" + paramType
			identName = "Has" + identName
		case condLike:
			identName = "Like" + identName
			condText = "LIKE"
		default:
			identName = "Eq" + identName
		}

		if i.Kind == condArr {
			sqlDBCond = append(sqlDBCond, fmt.Sprintf(`%s IN (VALUES "+placeholders%s+")`, dbName, paramName))
			sqlArrIdentArgs = append(sqlArrIdentArgs, paramName)
			sqlArrIdentArgsLen = append(sqlArrIdentArgsLen, fmt.Sprintf("len(%s)", paramName))
		} else {
			paramCount++
			sqlDBCond = append(sqlDBCond, fmt.Sprintf("%s %s $%d", dbName, condText, paramCount))
			sqlIdentArgs = append(sqlIdentArgs, paramName)
		}
		sqlIdentParams = append(sqlIdentParams, fmt.Sprintf("%s %s", paramName, paramType))
		sqlIdentNames = append(sqlIdentNames, identName)
	}
	return QueryCondSQLStrings{
		DBCond:          strings.Join(sqlDBCond, " AND "),
		IdentParams:     strings.Join(sqlIdentParams, ", "),
		IdentArgs:       strings.Join(sqlIdentArgs, ", "),
		ArrIdentArgs:    sqlArrIdentArgs,
		ArrIdentArgsLen: strings.Join(sqlArrIdentArgsLen, "+"),
		IdentNames:      strings.Join(sqlIdentNames, ""),
		ParamCount:      paramCount,
	}
}
