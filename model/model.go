package model

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io/fs"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
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
	// ErrorInvalidModel is returned when parsing a model with invalid syntax
	ErrorInvalidModel struct{}
)

func (e ErrorEnv) Error() string {
	return "Invalid execution environment"
}
func (e ErrorInvalidFile) Error() string {
	return "Invalid file"
}
func (e ErrorInvalidModel) Error() string {
	return "Invalid model"
}

type (
	dirObjPair struct {
		Dir gopackages.DirectiveInstance
		Obj gopackages.DirectiveObject
	}

	astField struct {
		Ident  string
		GoType string
		Tags   string
	}

	modelDef struct {
		Prefix   string
		Ident    string
		Fields   []modelField
		Indicies [][]modelField
		fieldMap map[string]modelField
	}

	modelField struct {
		Ident  string
		GoType string
		DBName string
		DBType string
		Num    int
	}

	queryDef struct {
		Ident       string
		Fields      []queryField
		QueryFields []queryField
	}

	queryField struct {
		Ident  string
		GoType string
		DBName string
		Num    int
		Mode   queryOpt
		Cond   []condField
	}

	condField struct {
		Kind  condType
		Field modelField
	}

	mainTemplateData struct {
		Generator string
		Version   string
		Package   string
	}

	modelTemplateData struct {
		Prefix     string
		ModelIdent string
		SQL        modelSQLStrings
	}

	modelSQLStrings struct {
		Setup            string
		DBNames          string
		Placeholders     string
		PlaceholderTpl   string
		PlaceholderCount string
		Idents           string
		Indicies         []modelIndex
		ColNum           string
	}

	modelIndex struct {
		Name    string
		Columns string
	}

	queryTemplateData struct {
		Prefix       string
		ModelIdent   string
		PrimaryField queryField
		SQL          querySQLStrings
		SQLCond      queryCondSQLStrings
	}

	querySQLStrings struct {
		DBNames      string
		Idents       string
		IdentRefs    string
		Placeholders string
		ColNum       string
	}

	queryCondSQLStrings struct {
		IdentNames      string
		IdentParams     string
		DBCond          string
		IdentArgs       string
		ArrIdentArgs    []string
		ArrIdentArgsLen string
		ParamCount      int
	}
)

type (
	Opts struct {
		Verbose        bool
		Version        string
		Output         string
		Include        string
		Ignore         string
		ModelDirective string
		QueryDirective string
		ModelTag       string
		QueryTag       string
	}

	ExecEnv struct {
		GoPackage string
	}
)

// Execute runs forge model generation
func Execute(opts Opts) error {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		return kerrors.WithKind(nil, ErrorEnv{}, "Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		return kerrors.WithKind(nil, ErrorEnv{}, "Environment variable GOFILE not provided by go generate")
	}

	log.Println(strings.Join([]string{
		"Generating model",
		fmt.Sprintf("Package: %s", gopackage),
		fmt.Sprintf("Source file: %s", gofile),
	}, "; "))

	return Generate(writefs.NewOSFS("."), os.DirFS("."), opts, ExecEnv{
		GoPackage: gopackage,
	})
}

func Generate(outputfs writefs.FS, inputfs fs.FS, opts Opts, env ExecEnv) error {
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

	astpkg, fset, err := gopackages.ReadDir(inputfs, includePattern, ignorePattern)
	if err != nil {
		return err
	}
	if astpkg.Name != env.GoPackage {
		return kerrors.WithKind(nil, ErrorEnv{}, "Environment variable GOPACKAGE does not match directory package")
	}

	directiveObjects := gopackages.FindDirectives(astpkg, []string{opts.ModelDirective, opts.QueryDirective})
	var modelObjects, queryObjects []dirObjPair
	for _, i := range directiveObjects {
		for _, j := range i.Directives {
			switch j.Sigil {
			case opts.ModelDirective:
				modelObjects = append(modelObjects, dirObjPair{
					Dir: j,
					Obj: i,
				})
			case opts.QueryDirective:
				queryObjects = append(queryObjects, dirObjPair{
					Dir: j,
					Obj: i,
				})
			}
		}
	}
	if len(modelObjects) == 0 {
		return kerrors.WithKind(nil, ErrorInvalidFile{}, "No models found")
	}

	modelDefs, err := parseModelDefinitions(modelObjects, opts.ModelTag, fset)
	if err != nil {
		return err
	}
	modelDefMap := map[string]modelDef{}
	for _, i := range modelDefs {
		modelDefMap[i.Prefix] = i
	}
	queryDefs, err := parseQueryDefinitions(queryObjects, opts.QueryTag, modelDefMap, fset)
	if err != nil {
		return err
	}

	tplmain, err := template.New("main").Parse(templateMain)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateMain")
	}
	tplmodel, err := template.New("model").Parse(templateModel)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateModel")
	}
	tplQuery := map[queryOpt]*template.Template{}
	tplQuery[queryOptGetOneEq], err = template.New("getoneeq").Parse(templateGetOneEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateGetOneEq")
	}
	tplQuery[queryOptGetGroup], err = template.New("getgroup").Parse(templateGetGroup)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateGetGroup")
	}
	tplQuery[queryOptGetGroupEq], err = template.New("getgroupeq").Parse(templateGetGroupEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateGetGroupEq")
	}
	tplQuery[queryOptUpdEq], err = template.New("updeq").Parse(templateUpdEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateUpdEq")
	}
	tplQuery[queryOptDelEq], err = template.New("deleq").Parse(templateDelEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateDelEq")
	}

	file, err := outputfs.OpenFile(opts.Output, generatedFileFlag, generatedFileMode)
	if err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to write file %s", opts.Output))
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close open file %s: %v\n", opts.Output, err)
		}
	}()
	fwriter := bufio.NewWriter(file)

	tplData := mainTemplateData{
		Generator: "go generate forge model",
		Version:   opts.Version,
		Package:   env.GoPackage,
	}
	if err := tplmain.Execute(fwriter, tplData); err != nil {
		return kerrors.WithMsg(err, "Failed to execute main model template")
	}

	for _, i := range modelDefs {
		if opts.Verbose {
			log.Printf("Detected model %s with fields:\n", i.Ident)
			for _, i := range i.Fields {
				log.Printf("* %s %s\n", i.Ident, i.GoType)
			}
		}
		tplData := modelTemplateData{
			Prefix:     i.Prefix,
			ModelIdent: i.Ident,
			SQL:        i.genModelSQL(),
		}
		if err := tplmodel.Execute(fwriter, tplData); err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to execute model template for struct %s", i.Ident))
		}
		for _, j := range queryDefs[i.Prefix] {
			if opts.Verbose {
				log.Printf("Detected model %s query %s with fields:", i.Ident, j.Ident)
				for _, i := range j.Fields {
					log.Printf("* %s %s\n", i.Ident, i.GoType)
				}
			}
			querySQLStrings := j.genQuerySQL()
			numFields := len(j.Fields)
			for _, k := range j.QueryFields {
				tplData := queryTemplateData{
					Prefix:       i.Prefix,
					ModelIdent:   j.Ident,
					PrimaryField: k,
					SQL:          querySQLStrings,
				}
				switch k.Mode {
				case queryOptGetOneEq:
					tplData.SQLCond = k.genQueryCondSQL(0)
				case queryOptGetGroupEq:
					tplData.SQLCond = k.genQueryCondSQL(2)
				case queryOptUpdEq:
					tplData.SQLCond = k.genQueryCondSQL(numFields)
				case queryOptDelEq:
					tplData.SQLCond = k.genQueryCondSQL(0)
				}
				if err := tplQuery[k.Mode].Execute(fwriter, tplData); err != nil {
					return kerrors.WithMsg(err, fmt.Sprintf("Failed to execute template for field %s on struct %s", tplData.PrimaryField.Ident, tplData.ModelIdent))
				}
			}
		}
	}

	if err := fwriter.Flush(); err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to write to file %s", opts.Output))
	}

	log.Printf("Generated file: %s\n", opts.Output)
	return nil
}

func (m *modelDef) genModelSQL() modelSQLStrings {
	colNum := len(m.Fields)
	sqlDefs := make([]string, 0, colNum)
	sqlDBNames := make([]string, 0, colNum)
	sqlPlaceholders := make([]string, 0, colNum)
	sqlPlaceholderTpl := make([]string, 0, colNum)
	sqlPlaceholderCount := make([]string, 0, colNum)
	sqlIdents := make([]string, 0, colNum)

	placeholderStart := 1
	for n, i := range m.Fields {
		sqlDefs = append(sqlDefs, fmt.Sprintf("%s %s", i.DBName, i.DBType))
		sqlDBNames = append(sqlDBNames, i.DBName)
		sqlPlaceholders = append(sqlPlaceholders, fmt.Sprintf("$%d", placeholderStart+n))
		sqlPlaceholderTpl = append(sqlPlaceholderTpl, "$%d")
		sqlPlaceholderCount = append(sqlPlaceholderCount, fmt.Sprintf("n+%d", placeholderStart+n))
		sqlIdents = append(sqlIdents, fmt.Sprintf("m.%s", i.Ident))
	}

	sqlIndicies := make([]modelIndex, 0, len(m.Indicies))
	for _, i := range m.Indicies {
		k := make([]string, 0, len(i))
		for _, j := range i {
			k = append(k, j.DBName)
		}
		sqlIndicies = append(sqlIndicies, modelIndex{
			Name:    strings.Join(k, "__"),
			Columns: strings.Join(k, ", "),
		})
	}

	return modelSQLStrings{
		Setup:            strings.Join(sqlDefs, ", "),
		DBNames:          strings.Join(sqlDBNames, ", "),
		Placeholders:     strings.Join(sqlPlaceholders, ", "),
		PlaceholderTpl:   strings.Join(sqlPlaceholderTpl, ", "),
		PlaceholderCount: strings.Join(sqlPlaceholderCount, ", "),
		Idents:           strings.Join(sqlIdents, ", "),
		Indicies:         sqlIndicies,
		ColNum:           strconv.Itoa(colNum),
	}
}

func (q *queryDef) genQuerySQL() querySQLStrings {
	colNum := len(q.Fields)
	sqlDBNames := make([]string, 0, colNum)
	sqlIdents := make([]string, 0, colNum)
	sqlIdentRefs := make([]string, 0, colNum)
	sqlPlaceholders := make([]string, 0, colNum)

	placeholderStart := 1
	for n, i := range q.Fields {
		sqlDBNames = append(sqlDBNames, i.DBName)
		sqlIdents = append(sqlIdents, fmt.Sprintf("m.%s", i.Ident))
		sqlIdentRefs = append(sqlIdentRefs, fmt.Sprintf("&m.%s", i.Ident))
		sqlPlaceholders = append(sqlPlaceholders, fmt.Sprintf("$%d", placeholderStart+n))
	}

	return querySQLStrings{
		DBNames:      strings.Join(sqlDBNames, ", "),
		Idents:       strings.Join(sqlIdents, ", "),
		IdentRefs:    strings.Join(sqlIdentRefs, ", "),
		Placeholders: strings.Join(sqlPlaceholders, ", "),
		ColNum:       fmt.Sprintf("%d", colNum),
	}
}

func (q *queryField) genQueryCondSQL(offset int) queryCondSQLStrings {
	sqlIdentNames := make([]string, 0, len(q.Cond))
	sqlIdentParams := make([]string, 0, len(q.Cond))
	sqlDBCond := make([]string, 0, len(q.Cond))
	sqlIdentArgs := make([]string, 0, len(q.Cond))
	sqlArrIdentArgs := make([]string, 0, len(q.Cond))
	sqlArrIdentArgsLen := make([]string, 0, len(q.Cond))
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
		case condIn:
			paramName = paramName + "s"
			paramType = "[]" + paramType
			identName = "Has" + identName
		case condLike:
			paramName = paramName + "Prefix"
			identName = "Like" + identName
			condText = "LIKE"
		default:
			identName = "Eq" + identName
		}

		sqlIdentNames = append(sqlIdentNames, identName)
		sqlIdentParams = append(sqlIdentParams, fmt.Sprintf("%s %s", paramName, paramType))
		if i.Kind == condIn {
			sqlDBCond = append(sqlDBCond, fmt.Sprintf(`%s IN (VALUES "+placeholders%s+")`, dbName, paramName))
			sqlArrIdentArgs = append(sqlArrIdentArgs, paramName)
			sqlArrIdentArgsLen = append(sqlArrIdentArgsLen, fmt.Sprintf("len(%s)", paramName))
		} else {
			paramCount++
			sqlDBCond = append(sqlDBCond, fmt.Sprintf("%s %s $%d", dbName, condText, paramCount))
			sqlIdentArgs = append(sqlIdentArgs, paramName)
		}
	}
	return queryCondSQLStrings{
		IdentNames:      strings.Join(sqlIdentNames, ""),
		IdentParams:     strings.Join(sqlIdentParams, ", "),
		DBCond:          strings.Join(sqlDBCond, " AND "),
		IdentArgs:       strings.Join(sqlIdentArgs, ", "),
		ArrIdentArgs:    sqlArrIdentArgs,
		ArrIdentArgsLen: strings.Join(sqlArrIdentArgsLen, "+"),
		ParamCount:      paramCount,
	}
}

func parseModelDefinitions(modelObjects []dirObjPair, modelTag string, fset *token.FileSet) ([]modelDef, error) {
	modelDefs := make([]modelDef, 0, len(modelObjects))

	for _, i := range modelObjects {
		dirargs := strings.Fields(i.Dir.Directive)
		if len(dirargs) != 2 || dirargs[1] == "" {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Model directive without prefix")
		}
		prefix := dirargs[1]
		if i.Obj.Kind != gopackages.ObjKindDeclType {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Model directive used on non-type declaration")
		}
		typeSpec, ok := i.Obj.Obj.(*ast.TypeSpec)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Unexpected directive object type")
		}
		structName := typeSpec.Name.Name
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Model directive used on non-struct type declaration")
		}
		if structType.Incomplete {
			return nil, kerrors.WithMsg(nil, "Unexpected incomplete struct definition")
		}
		astFields, err := findFields(modelTag, structType, fset)
		if err != nil {
			return nil, err
		}
		if len(astFields) == 0 {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "No model fields found on struct")
		}
		modelFields, fieldMap, indicies, err := parseModelFields(astFields)
		if err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse model fields for struct %s", structName))
		}
		modelDefs = append(modelDefs, modelDef{
			Prefix:   prefix,
			Ident:    structName,
			Fields:   modelFields,
			Indicies: indicies,
			fieldMap: fieldMap,
		})
	}

	return modelDefs, nil
}

func parseModelFields(astfields []astField) ([]modelField, map[string]modelField, [][]modelField, error) {
	fields := make([]modelField, 0, len(astfields))
	seenFields := map[string]modelField{}
	var tagIndicies [][]string

	for n, i := range astfields {
		dbstr, rest, _ := strings.Cut(i.Tags, ";")
		dbName, dbType, ok := strings.Cut(dbstr, ",")
		if !ok || dbName == "" || dbType == "" {
			return nil, nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Model field tag must be dbname,dbtype[;opt[,fields ...][; ...]] on field %s", i.Ident))
		}
		if dup, ok := seenFields[dbName]; ok {
			return nil, nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Duplicate field %s on %s and %s", dbName, i.Ident, dup.Ident))
		}
		f := modelField{
			Ident:  i.Ident,
			GoType: i.GoType,
			DBName: dbName,
			DBType: dbType,
			Num:    n + 1,
		}
		seenFields[dbName] = f
		fields = append(fields, f)
		var opts []string
		if rest != "" {
			opts = strings.Split(rest, ";")
		}
		for _, i := range opts {
			opt := strings.Split(i, ",")
			optflag, err := parseModelOpt(opt[0])
			if err != nil {
				return nil, nil, nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse model opt for field %s", f.Ident))
			}
			switch optflag {
			case modelOptIndex:
				args := make([]string, len(opt))
				copy(args, opt[1:])
				args[len(args)-1] = dbName
				tagIndicies = append(tagIndicies, args)
			}
		}
	}

	indicies := [][]modelField{}
	for _, i := range tagIndicies {
		k := make([]modelField, 0, len(i))
		for _, j := range i {
			f, ok := seenFields[j]
			if !ok {
				return nil, nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("No field %s for index", j))
			}
			k = append(k, f)
		}
		indicies = append(indicies, k)
	}

	return fields, seenFields, indicies, nil
}

type (
	modelOpt int
)

const (
	modelOptUnknown modelOpt = iota
	modelOptIndex
)

func parseModelOpt(opt string) (modelOpt, error) {
	switch opt {
	case "index":
		return modelOptIndex, nil
	default:
		return modelOptUnknown, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Illegal opt %s", opt))
	}
}

func parseQueryDefinitions(queryObjects []dirObjPair, queryTag string, modelDefs map[string]modelDef, fset *token.FileSet) (map[string][]queryDef, error) {
	queryDefs := map[string][]queryDef{}

	for _, i := range queryObjects {
		dirargs := strings.Fields(i.Dir.Directive)
		if len(dirargs) != 2 || dirargs[1] == "" {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Query directive without prefix")
		}
		prefix := dirargs[1]
		mdef, ok := modelDefs[prefix]
		if !ok {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, fmt.Sprintf("Query directive prefix %s without model definition", prefix))
		}
		if i.Obj.Kind != gopackages.ObjKindDeclType {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Query directive used on non-type declaration")
		}
		typeSpec, ok := i.Obj.Obj.(*ast.TypeSpec)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Unexpected directive object type")
		}
		structName := typeSpec.Name.Name
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "Query directive used on non-struct type declaration")
		}
		if structType.Incomplete {
			return nil, kerrors.WithMsg(nil, "Unexpected incomplete struct definition")
		}
		astFields, err := findFields(queryTag, structType, fset)
		if err != nil {
			return nil, err
		}
		if len(astFields) == 0 {
			return nil, kerrors.WithKind(nil, ErrorInvalidFile{}, "No query fields found on struct")
		}
		fields, queries, err := parseQueryFields(astFields, mdef.fieldMap)
		if err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse query fields for struct %s", structName))
		}
		queryDefs[prefix] = append(queryDefs[prefix], queryDef{
			Ident:       structName,
			Fields:      fields,
			QueryFields: queries,
		})
	}

	return queryDefs, nil
}

func parseQueryFields(astfields []astField, fieldMap map[string]modelField) ([]queryField, []queryField, error) {
	var fields []queryField
	var queryFields []queryField

	for n, i := range astfields {
		dbName, rest, _ := strings.Cut(i.Tags, ";")
		if len(dbName) < 1 {
			return nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Query field tag must be dbname[;flag[,args ...][; ...]] for field %s", i.Ident))
		}
		if mfield, ok := fieldMap[dbName]; !ok || i.GoType != mfield.GoType {
			return nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Field %s with type %s does not exist on model", dbName, i.GoType))
		}
		f := queryField{
			Ident:  i.Ident,
			GoType: i.GoType,
			DBName: dbName,
			Num:    n + 1,
		}
		fields = append(fields, f)
		if rest != "" {
			for _, t := range strings.Split(rest, ";") {
				opt := strings.Split(t, ",")
				optflag, err := parseQueryOpt(opt[0])
				if err != nil {
					return nil, nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse query opt for field %s", f.Ident))
				}
				f.Mode = optflag
				switch optflag {
				case queryOptGetOneEq, queryOptGetGroupEq, queryOptUpdEq, queryOptDelEq:
					{
						if len(opt) < 2 {
							return nil, nil, kerrors.WithKind(err, ErrorInvalidModel{}, fmt.Sprintf("Query field opt must be dbname;flag,fields,... for opt %s on field %s", opt[0], f.Ident))
						}
						args := opt[1:]
						k := make([]condField, 0, len(args))
						for _, cond := range args {
							fieldName, cond, err := parseCondField(cond)
							if err != nil {
								return nil, nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to parse condition field for opt %s on field %s", opt[0], f.Ident))
							}
							if field, ok := fieldMap[fieldName]; ok {
								k = append(k, condField{
									Kind:  cond,
									Field: field,
								})
							} else {
								return nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Invalid condition field %s for field %s", fieldName, i.Ident))
							}
						}
						f.Cond = k
					}
				default:
					if len(opt) != 1 {
						return nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Field tag must be dbname;flag for opt %s on field %s", opt[0], i.Ident))
					}
				}
				queryFields = append(queryFields, f)
			}
		}
	}

	if len(queryFields) == 0 {
		return nil, nil, kerrors.WithKind(nil, ErrorInvalidModel{}, "Query does not contain a query field")
	}

	return fields, queryFields, nil
}

type (
	queryOpt int
)

const (
	queryOptUnknown queryOpt = iota
	queryOptGetOneEq
	queryOptGetGroup
	queryOptGetGroupEq
	queryOptUpdEq
	queryOptDelEq
)

func parseQueryOpt(opt string) (queryOpt, error) {
	switch opt {
	case "getoneeq":
		return queryOptGetOneEq, nil
	case "getgroup":
		return queryOptGetGroup, nil
	case "getgroupeq":
		return queryOptGetGroupEq, nil
	case "updeq":
		return queryOptUpdEq, nil
	case "deleq":
		return queryOptDelEq, nil
	default:
		return queryOptUnknown, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Illegal opt %s", opt))
	}
}

type (
	condType int
)

const (
	condUnknown condType = iota
	condEq
	condNeq
	condLt
	condLeq
	condGt
	condGeq
	condIn
	condLike
)

func parseCondField(field string) (string, condType, error) {
	fieldName, condName, _ := strings.Cut(field, "|")
	if condName != "" {
		cond, err := parseCond(condName)
		if err != nil {
			return "", condUnknown, err
		}
		return fieldName, cond, nil
	}
	return fieldName, condEq, nil
}

func parseCond(cond string) (condType, error) {
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
	case "in":
		return condIn, nil
	case "like":
		return condLike, nil
	default:
		return condUnknown, kerrors.WithKind(nil, ErrorInvalidModel{}, fmt.Sprintf("Illegal cond type %s", cond))
	}
}

func findFields(tagName string, structType *ast.StructType, fset *token.FileSet) ([]astField, error) {
	var fields []astField
	for _, field := range structType.Fields.List {
		if field.Tag == nil {
			continue
		}
		structTag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		tagVal, ok := structTag.Lookup(tagName)
		if !ok {
			continue
		}

		if len(field.Names) != 1 {
			return nil, kerrors.WithKind(nil, ErrorInvalidModel{}, "Only one field allowed per tag")
		}

		ident := field.Names[0].Name

		goType := bytes.Buffer{}
		if err := printer.Fprint(&goType, fset, field.Type); err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Failed to print go struct field type for field %s", ident))
		}

		m := astField{
			Ident:  ident,
			GoType: goType.String(),
			Tags:   tagVal,
		}
		fields = append(fields, m)
	}
	return fields, nil
}
