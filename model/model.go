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
		Ident      string
		Fields     []ModelField
		PrimaryKey ModelField
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
		Mode   int
		Cond   []ModelField
	}

	ModelSQLStrings struct {
		Setup            string
		DBNames          string
		Placeholders     string
		PlaceholderTpl   string
		PlaceholderCount string
		Idents           string
		IdentRefs        string
		ColNum           string
	}

	ModelTemplateData struct {
		Generator  string
		Package    string
		Prefix     string
		TableName  string
		Imports    string
		ModelIdent string
		PrimaryKey ModelField
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
		DBCond      string
		IdentParams string
		IdentArgs   string
		IdentNames  string
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

func Execute(verbose bool, generatedFilepath, prefix, tableName, modelIdent string, queryIdents []string) {
	gopackage := os.Getenv("GOPACKAGE")
	if len(gopackage) == 0 {
		log.Fatal("Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if len(gofile) == 0 {
		log.Fatal("Environment variable GOPACKAGE not provided by go generate")
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
		log.Fatal(err)
	}

	tplquery, err := template.New("querysingle").Parse(templateQuerySingle)
	if err != nil {
		log.Fatal(err)
	}

	tplquerygroup, err := template.New("querygroup").Parse(templateQueryGroup)
	if err != nil {
		log.Fatal(err)
	}

	tplquerygroupeq, err := template.New("groupeq").Parse(templateQueryGroupEq)
	if err != nil {
		log.Fatal(err)
	}

	tplquerygroupset, err := template.New("groupset").Parse(templateQueryGroupSet)
	if err != nil {
		log.Fatal(err)
	}

	tplupdgroupeq, err := template.New("updgroupeq").Parse(templateUpdGroupEq)
	if err != nil {
		log.Fatal(err)
	}

	tplupdgroupset, err := template.New("updgroupset").Parse(templateUpdGroupSet)
	if err != nil {
		log.Fatal(err)
	}

	tpldelgroupeq, err := template.New("delgroupeq").Parse(templateDelGroupEq)
	if err != nil {
		log.Fatal(err)
	}

	tpldelgroupset, err := template.New("delgroupset").Parse(templateDelGroupSet)
	if err != nil {
		log.Fatal(err)
	}

	genfile, err := os.OpenFile(generatedFilepath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer genfile.Close()
	genFileWriter := bufio.NewWriter(genfile)

	tplData := ModelTemplateData{
		Generator:  "go generate",
		Package:    gopackage,
		Prefix:     prefix,
		TableName:  tableName,
		Imports:    deps,
		ModelIdent: modelDef.Ident,
		PrimaryKey: modelDef.PrimaryKey,
		SQL:        modelDef.genModelSQL(),
	}
	if err := tplmodel.Execute(genFileWriter, tplData); err != nil {
		log.Fatal(err)
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
			case flagGet:
				if err := tplquery.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagGetGroup:
				if err := tplquerygroup.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagGetGroupEq:
				tplData.SQLCond = i.genQueryCondSQL(2)
				if err := tplquerygroupeq.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagGetGroupSet:
				if err := tplquerygroupset.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagUpdGroupEq:
				tplData.SQLCond = i.genQueryCondSQL(len(queryDef.Fields))
				if err := tplupdgroupeq.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagUpdGroupSet:
				if err := tplupdgroupset.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagDelGroupEq:
				tplData.SQLCond = i.genQueryCondSQL(0)
				if err := tpldelgroupeq.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
				}
			case flagDelGroupSet:
				if err := tpldelgroupset.Execute(genFileWriter, tplData); err != nil {
					log.Fatal(err)
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
		log.Fatal(err)
	}
	if root.Decls == nil {
		log.Fatal("No top level declarations")
	}

	modelFields, primaryField, seenFields := parseModelFields(findFields(modelTagName, findStruct(modelIdent, root.Decls), fset))

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
		Ident:      modelIdent,
		Fields:     modelFields,
		PrimaryKey: primaryField,
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

	log.Fatal(ident + " struct not found")
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

func parseModelFields(astfields []ASTField) ([]ModelField, ModelField, map[string]ModelField) {
	hasPK := false
	var primaryKey ModelField

	seenFields := map[string]ModelField{}

	fields := []ModelField{}
	for n, i := range astfields {
		tags := strings.Split(i.Tags, ",")
		if len(tags) != 2 {
			log.Fatal("Model field tag must be dbname,dbtype")
		}
		dbName := tags[0]
		dbType := tags[1]
		if len(dbName) == 0 {
			log.Fatal(i.Ident + " dbname not set")
		}
		if len(dbType) == 0 {
			log.Fatal(i.Ident + " dbtype not set")
		}
		if _, ok := seenFields[dbName]; ok {
			log.Fatal("Duplicate field " + dbName)
		}
		f := ModelField{
			Ident:  i.Ident,
			GoType: i.GoType,
			DBName: dbName,
			DBType: dbType,
			Num:    n + 1,
		}
		seenFields[dbName] = f
		if strings.Contains(dbType, "PRIMARY KEY") {
			if hasPK {
				log.Fatal("Model cannot contain two primary keys")
			}
			hasPK = true
			primaryKey = f
		}
		fields = append(fields, f)
	}

	if !hasPK {
		log.Fatal("Model does not contain a primary key")
	}

	return fields, primaryKey, seenFields
}

func parseQueryFields(astfields []ASTField, seenFields map[string]ModelField) ([]QueryField, []QueryField, string) {
	hasQF := false
	queryFields := []QueryField{}
	deps := Dependencies{}

	fields := []QueryField{}
	for n, i := range astfields {
		props := strings.SplitN(i.Tags, ",", 2)
		if len(props) < 1 {
			log.Fatal("Field tag must be dbname[,flag[,args ...][; ...]]")
		}
		dbName := props[0]
		modelField, ok := seenFields[dbName]
		if !ok || i.GoType != modelField.GoType {
			log.Fatal("Field " + dbName + " with type " + i.GoType + " does not exist on model")
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
				case flagGetGroupEq:
					if len(tags) < 2 {
						log.Fatal("Field tag must be dbname,flag,eqcond,... for field " + i.Ident)
					}
					k := make([]ModelField, 0, len(tags[1:]))
					for _, cond := range tags[1:] {
						if modelField, ok := seenFields[cond]; ok {
							k = append(k, modelField)
						} else {
							log.Fatal("Invalid eq condition field for field " + i.Ident)
						}
					}
					f.Cond = k
				case flagUpdGroupEq:
					if len(tags) < 2 {
						log.Fatal("Field tag must be dbname,flag,eqcond,... for field " + i.Ident)
					}
					k := make([]ModelField, 0, len(tags[1:]))
					for _, cond := range tags[1:] {
						if modelField, ok := seenFields[cond]; ok {
							k = append(k, modelField)
						} else {
							log.Fatal("Invalid eq condition field for field " + i.Ident)
						}
					}
					f.Cond = k
				case flagDelGroupEq:
					if len(tags) < 2 {
						log.Fatal("Field tag must be dbname,flag,eqcond,... for field " + i.Ident)
					}
					k := make([]ModelField, 0, len(tags[1:]))
					for _, cond := range tags[1:] {
						if modelField, ok := seenFields[cond]; ok {
							k = append(k, modelField)
						} else {
							log.Fatal("Invalid eq condition field for field " + i.Ident)
						}
					}
					f.Cond = k
				default:
					if len(tags) != 1 {
						log.Fatal("Field tag must be dbname,flag for field " + i.Ident)
					}
				}
				queryFields = append(queryFields, f)
			}
		}
	}

	if !hasQF {
		log.Fatal("Query does not contain a query field")
	}

	return fields, queryFields, deps.String()
}

const (
	flagGet = iota
	flagGetGroup
	flagGetGroupEq
	flagGetGroupSet
	flagUpdGroupEq
	flagUpdGroupSet
	flagDelGroupEq
	flagDelGroupSet
)

func parseFlag(flag string) int {
	switch flag {
	case "get":
		return flagGet
	case "getgroup":
		return flagGetGroup
	case "getgroupeq":
		return flagGetGroupEq
	case "getgroupset":
		return flagGetGroupSet
	case "updgroupeq":
		return flagUpdGroupEq
	case "updgroupset":
		return flagUpdGroupSet
	case "delgroupeq":
		return flagDelGroupEq
	case "delgroupset":
		return flagDelGroupSet
	default:
		log.Fatal("Illegal flag " + flag)
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

	return ModelSQLStrings{
		Setup:            strings.Join(sqlDefs, ", "),
		DBNames:          strings.Join(sqlDBNames, ", "),
		Placeholders:     strings.Join(sqlPlaceholders, ", "),
		PlaceholderTpl:   strings.Join(sqlPlaceholderTpl, ", "),
		PlaceholderCount: strings.Join(sqlPlaceholderCount, ", "),
		Idents:           strings.Join(sqlIdents, ", "),
		IdentRefs:        strings.Join(sqlIdentRefs, ", "),
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
	sqlIdentNames := make([]string, 0, len(q.Cond))
	placeholderStart := 1
	for n, i := range q.Cond {
		paramName := strings.ToLower(i.Ident)
		sqlDBCond = append(sqlDBCond, fmt.Sprintf("%s = $%d", i.DBName, n+offset+placeholderStart))
		sqlIdentParams = append(sqlIdentParams, fmt.Sprintf("%s %s", paramName, i.GoType))
		sqlIdentArgs = append(sqlIdentArgs, paramName)
		sqlIdentNames = append(sqlIdentNames, i.Ident)
	}
	return QueryCondSQLStrings{
		DBCond:      strings.Join(sqlDBCond, " AND "),
		IdentParams: strings.Join(sqlIdentParams, ", "),
		IdentArgs:   strings.Join(sqlIdentArgs, ", "),
		IdentNames:  strings.Join(sqlIdentNames, ""),
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
