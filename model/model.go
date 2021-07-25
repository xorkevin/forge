package model

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
	"sort"
	"strings"
	"text/template"
)

const (
	modelTagName = "model"
	queryTagName = "query"
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
		TableName  string
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
		TableName    string
		ModelIdent   string
		PrimaryField QueryField
		SQL          QuerySQLStrings
		SQLCond      QueryCondSQLStrings
	}
)

func Execute(verbose bool, version, generatedFilepath, prefix, tableName, modelIdent string, queryIdents []string) {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		log.Fatalln("Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		log.Fatalln("Environment variable GOPACKAGE not provided by go generate")
	}

	fmt.Println(strings.Join([]string{
		"Generating model",
		fmt.Sprintf("Package: %s", gopackage),
		fmt.Sprintf("Source file: %s", gofile),
		fmt.Sprintf("Table name: %s", tableName),
		fmt.Sprintf("Model ident: %s", modelIdent),
		fmt.Sprintf("Additional queries: %s", strings.Join(queryIdents, ", ")),
	}, "; "))

	modelDef, queryDefs, deps := parseDefinitions(gofile, modelIdent, queryIdents)

	tplmodel, err := template.New("model").Parse(templateModel)
	if err != nil {
		log.Fatalln(err)
	}

	tplgetoneeq, err := template.New("getoneeq").Parse(templateGetOneEq)
	if err != nil {
		log.Fatalln(err)
	}

	tplgetgroup, err := template.New("getgroup").Parse(templateGetGroup)
	if err != nil {
		log.Fatalln(err)
	}

	tplgetgroupeq, err := template.New("getgroupeq").Parse(templateGetGroupEq)
	if err != nil {
		log.Fatalln(err)
	}

	tplupdeq, err := template.New("updeq").Parse(templateUpdEq)
	if err != nil {
		log.Fatalln(err)
	}

	tpldeleq, err := template.New("deleq").Parse(templateDelEq)
	if err != nil {
		log.Fatalln(err)
	}

	genfile, err := os.OpenFile(generatedFilepath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalln(err)
	}
	defer genfile.Close()
	genFileWriter := bufio.NewWriter(genfile)

	tplData := ModelTemplateData{
		Generator:  "go generate forge model",
		Version:    version,
		Package:    gopackage,
		Prefix:     prefix,
		TableName:  tableName,
		Imports:    deps,
		ModelIdent: modelDef.Ident,
		SQL:        modelDef.genModelSQL(),
	}
	if err := tplmodel.Execute(genFileWriter, tplData); err != nil {
		log.Fatalln(err)
	}

	if verbose {
		fmt.Println("Detected model fields:")
		for _, i := range modelDef.Fields {
			fmt.Printf("- %s %s\n", i.Ident, i.GoType)
		}
	}

	for _, queryDef := range queryDefs {
		if verbose {
			fmt.Println("Detected query " + queryDef.Ident + " fields:")
			for _, i := range queryDef.Fields {
				fmt.Printf("- %s %s\n", i.Ident, i.GoType)
			}
		}
		querySQLStrings := queryDef.genQuerySQL()
		for _, i := range queryDef.QueryFields {
			tplData := QueryTemplateData{
				Prefix:       prefix,
				TableName:    tableName,
				ModelIdent:   queryDef.Ident,
				PrimaryField: i,
				SQL:          querySQLStrings,
			}
			switch i.Mode {
			case flagGetOneEq:
				tplData.SQLCond = i.genQueryCondSQL(0)
				if err := tplgetoneeq.Execute(genFileWriter, tplData); err != nil {
					log.Fatalln(err)
				}
			case flagGetGroup:
				if err := tplgetgroup.Execute(genFileWriter, tplData); err != nil {
					log.Fatalln(err)
				}
			case flagGetGroupEq:
				tplData.SQLCond = i.genQueryCondSQL(2)
				if err := tplgetgroupeq.Execute(genFileWriter, tplData); err != nil {
					log.Fatalln(err)
				}
			case flagUpdEq:
				tplData.SQLCond = i.genQueryCondSQL(len(queryDef.Fields))
				if err := tplupdeq.Execute(genFileWriter, tplData); err != nil {
					log.Fatalln(err)
				}
			case flagDelEq:
				tplData.SQLCond = i.genQueryCondSQL(0)
				if err := tpldeleq.Execute(genFileWriter, tplData); err != nil {
					log.Fatalln(err)
				}
			}
		}
	}

	genFileWriter.Flush()

	fmt.Printf("Generated file: %s\n", generatedFilepath)
}

func parseDefinitions(gofile string, modelIdent string, queryIdents []string) (ModelDef, []QueryDef, string) {
	fset := token.NewFileSet()
	root, err := parser.ParseFile(fset, gofile, nil, parser.AllErrors)
	if err != nil {
		log.Fatalln(err)
	}
	if root.Decls == nil {
		log.Fatalln("No top level declarations")
	}

	modelFields, seenFields, indicies := parseModelFields(findFields(modelTagName, findStruct(modelIdent, root.Decls), fset))

	deps := Dependencies{}
	queryDefs := []QueryDef{}
	for _, ident := range queryIdents {
		fields, queries, d := parseQueryFields(findFields(queryTagName, findStruct(ident, root.Decls), fset), seenFields)
		queryDefs = append(queryDefs, QueryDef{
			Ident:       ident,
			Fields:      fields,
			QueryFields: queries,
		})
		deps.Add(d)
	}

	return ModelDef{
		Ident:    modelIdent,
		Fields:   modelFields,
		Indicies: indicies,
	}, queryDefs, deps.String()
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

	log.Fatalf("Struct %s not found\n", ident)
	return nil
}

func findFields(tagName string, modelDef *ast.StructType, fset *token.FileSet) []ASTField {
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

		goType := bytes.Buffer{}
		if err := printer.Fprint(&goType, fset, field.Type); err != nil {
			log.Fatalln(err)
		}

		if len(field.Names) != 1 {
			log.Fatalln("Only one field allowed per tag")
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

func parseModelFields(astfields []ASTField) ([]ModelField, map[string]ModelField, [][]ModelField) {
	seenFields := map[string]ModelField{}
	tagIndicies := [][]string{}

	fields := []ModelField{}
	for n, i := range astfields {
		tags := strings.SplitN(i.Tags, ",", 2)
		if len(tags) < 2 {
			log.Fatalln("Model field tag must be dbname,dbtype[;opt[,fields ...][; ...]]")
		}
		dbName := tags[0]
		opts := strings.Split(tags[1], ";")
		dbType := opts[0]
		if len(dbName) == 0 {
			log.Fatalf("%s dbname not set\n", i.Ident)
		}
		if len(dbType) == 0 {
			log.Fatalf("%s dbtype not set\n", i.Ident)
		}
		if _, ok := seenFields[dbName]; ok {
			log.Fatalf("Duplicate field %s\n", dbName)
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
			opt := parseModelOpt(tags[0])
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
				log.Fatalf("Invalid index field %s for field %s\n", j, i[len(i)-1])
			}
			k = append(k, f)
		}
		indicies = append(indicies, k)
	}

	return fields, seenFields, indicies
}

func parseQueryFields(astfields []ASTField, seenFields map[string]ModelField) ([]QueryField, []QueryField, string) {
	hasQF := false
	queryFields := []QueryField{}
	deps := Dependencies{}

	fields := []QueryField{}
	for n, i := range astfields {
		props := strings.SplitN(i.Tags, ";", 2)
		if len(props) < 1 {
			log.Fatalln("Field tag must be dbname[;flag[,args ...][; ...]]")
		}
		dbName := props[0]
		modelField, ok := seenFields[dbName]
		if !ok || i.GoType != modelField.GoType {
			log.Fatalf("Field %s with type %s does not exist on model\n", dbName, i.GoType)
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
				tagflag := parseFlag(tags[0])
				f.Mode = tagflag
				switch tagflag {
				case flagGetOneEq, flagGetGroupEq, flagUpdEq, flagDelEq:
					if len(tags) < 2 {
						log.Fatalf("Field tag must be dbname;flag,fields,... for field %s\n", i.Ident)
					}
					k := make([]CondField, 0, len(tags[1:]))
					for _, cond := range tags[1:] {
						condName, kind := parseCondField(cond)
						if field, ok := seenFields[condName]; ok {
							k = append(k, CondField{
								Kind:  kind,
								Field: field,
							})
						} else {
							log.Fatalf("Invalid condition field %s for field %s\n", condName, i.Ident)
						}
					}
					f.Cond = k
				default:
					if len(tags) != 1 {
						log.Fatalf("Field tag must be dbname,flag for field %s\n", i.Ident)
					}
				}
				queryFields = append(queryFields, f)
			}
		}
	}

	if !hasQF {
		log.Fatalln("Query does not contain a query field")
	}

	return fields, queryFields, deps.String()
}

type (
	ModelOpt int
)

const (
	optIndex ModelOpt = iota
)

func parseModelOpt(opt string) ModelOpt {
	switch opt {
	case "index":
		return optIndex
	default:
		log.Fatalf("Illegal opt %s\n", opt)
	}
	return -1
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

func parseFlag(flag string) QueryFlag {
	switch flag {
	case "getoneeq":
		return flagGetOneEq
	case "getgroup":
		return flagGetGroup
	case "getgroupeq":
		return flagGetGroupEq
	case "updeq":
		return flagUpdEq
	case "deleq":
		return flagDelEq
	default:
		log.Fatalf("Illegal flag %s\n", flag)
	}
	return -1
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

func parseCondField(field string) (string, CondType) {
	k := strings.SplitN(field, "|", 2)
	if len(k) == 2 {
		return k[0], parseCond(k[1])
	}
	return field, condEq
}

func parseCond(cond string) CondType {
	switch cond {
	case "eq":
		return condEq
	case "neq":
		return condNeq
	case "lt":
		return condLt
	case "leq":
		return condLeq
	case "gt":
		return condGt
	case "geq":
		return condGeq
	case "arr":
		return condArr
	case "like":
		return condLike
	default:
		log.Fatalf("Illegal cond type %s\n", cond)
	}
	return -1
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

type (
	Dependencies map[string]struct{}
)

func (d Dependencies) Add(deps string) {
	for _, i := range strings.Fields(deps) {
		d[i] = struct{}{}
	}
}

func (d Dependencies) String() string {
	if len(d) == 0 {
		return ""
	}
	k := make([]string, 0, len(d))
	for i := range d {
		k = append(k, i)
	}
	sort.Strings(k)
	return "\n" + strings.Join(k, "\n") + "\n"
}
