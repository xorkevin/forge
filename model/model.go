package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"strconv"
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
	// ErrInvalidSchema is returned when parsing an invalid schema file
	ErrInvalidSchema errInvalidSchema
	// ErrInvalidFile is returned when parsing an invalid model file
	ErrInvalidFile errInvalidFile
	// ErrInvalidModel is returned when checking an invalid model
	ErrInvalidModel errInvalidModel
)

type (
	errEnv           struct{}
	errInvalidFile   struct{}
	errInvalidModel  struct{}
	errInvalidSchema struct{}
)

func (e errEnv) Error() string {
	return "Invalid execution environment"
}

func (e errInvalidFile) Error() string {
	return "Invalid file"
}

func (e errInvalidModel) Error() string {
	return "Invalid model"
}

func (e errInvalidSchema) Error() string {
	return "Invalid schema"
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

	modelIndexOrderOpt struct {
		Col string `json:"col"`
		Dir string `json:"dir"`
	}

	modelIndexOpts struct {
		Name    string               `json:"name"`
		Columns []modelIndexOrderOpt `json:"columns"`
	}

	modelConstraintOpts struct {
		Kind    string   `json:"kind"`
		Columns []string `json:"columns"`
	}

	modelOpts struct {
		Setup       string                `json:"setup"`
		Constraints []modelConstraintOpts `json:"constraints"`
		Indicies    []modelIndexOpts      `json:"indicies"`
	}

	queryCondOpt struct {
		Col  string `json:"col"`
		Cond string `json:"cond"`
	}

	queryOrderOpt struct {
		Col string `json:"col"`
		Dir string `json:"dir"`
	}

	queryOpts struct {
		Kind       string          `json:"kind"`
		Name       string          `json:"name"`
		Conditions []queryCondOpt  `json:"conditions"`
		Order      []queryOrderOpt `json:"order"`
	}

	modelConfig struct {
		Model   modelOpts              `json:"model"`
		Queries map[string][]queryOpts `json:"queries"`
	}

	modelSchema struct {
		Models map[string]modelConfig `json:"models"`
	}

	modelDef struct {
		Prefix      string
		Ident       string
		Fields      []modelField
		Constraints []modelConstraint
		Indicies    []modelIndexDef
		opts        modelOpts
		fieldMap    map[string]modelField
	}

	modelField struct {
		Ident  string
		GoType string
		DBName string
		DBType string
		Num    int
	}

	modelConstraint struct {
		Kind    string
		Columns []modelField
	}

	modelIndexDef struct {
		Name    string
		Columns []modelIndexColumn
	}

	modelIndexColumn struct {
		Field modelField
		Dir   string
	}

	queryGroupDef struct {
		Ident   string
		Fields  []queryField
		Queries []queryDef
	}

	queryField struct {
		Ident  string
		GoType string
		DBName string
		Num    int
	}

	queryDef struct {
		Kind  queryKind
		Name  string
		Conds []queryCondField
		Order []queryOrderField
	}

	queryCondField struct {
		Kind  condType
		Field modelField
	}

	queryOrderField struct {
		Field modelField
		Dir   string
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
		Prefix     string
		ModelIdent string
		Name       string
		SQL        querySQLStrings
		SQLCond    queryCondSQLStrings
		SQLOrder   queryOrderSQLStrings
	}

	querySQLStrings struct {
		DBNames      string
		NumDBNames   int
		Idents       string
		IdentRefs    string
		Placeholders string
		ColNum       string
	}

	queryCondSQLStrings struct {
		IdentParams     string
		DBCond          string
		IdentArgs       string
		ArrIdentArgs    []string
		ArrIdentArgsLen string
		ParamCount      int
	}

	queryOrderSQLStrings struct {
		DBOrder string
	}
)

type (
	Opts struct {
		Output         string
		Schema         string
		Include        string
		Ignore         string
		ModelDirective string
		QueryDirective string
		ModelTag       string
	}

	ExecEnv struct {
		GoPackage string
	}
)

// Execute runs forge model generation
func Execute(log klog.Logger, version string, opts Opts) error {
	gopackage := os.Getenv("GOPACKAGE")
	if gopackage == "" {
		return kerrors.WithKind(nil, ErrEnv, "Environment variable GOPACKAGE not provided by go generate")
	}
	gofile := os.Getenv("GOFILE")
	if gofile == "" {
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

	var schema modelSchema
	if opts.Schema != "" {
		if f, err := fs.ReadFile(inputfs, opts.Schema); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return kerrors.WithMsg(err, fmt.Sprintf("Failed reading schema file: %s", opts.Schema))
			}
		} else {
			if err := json.Unmarshal(f, &schema); err != nil {
				return kerrors.WithKind(err, ErrInvalidSchema, fmt.Sprintf("Invalid schema file: %s", opts.Schema))
			}
		}
	}

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
		return kerrors.WithKind(nil, ErrEnv, "Environment variable GOPACKAGE does not match directory package")
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
		return kerrors.WithKind(nil, ErrInvalidFile, "No models found")
	}

	modelDefs, err := parseModelDefinitions(modelObjects, opts.ModelTag, fset, schema)
	if err != nil {
		return err
	}
	modelDefMap := map[string]modelDef{}
	for _, i := range modelDefs {
		modelDefMap[i.Prefix] = i
	}
	queryGroupDefs, err := parseQueryDefinitions(queryObjects, opts.ModelTag, modelDefMap, fset, schema)
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
	tplQuery := map[queryKind]*template.Template{}
	tplQuery[queryKindGetOneEq], err = template.New("getoneeq").Parse(templateGetOneEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateGetOneEq")
	}
	tplQuery[queryKindGetGroup], err = template.New("getgroup").Parse(templateGetGroup)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateGetGroup")
	}
	tplQuery[queryKindGetGroupEq], err = template.New("getgroupeq").Parse(templateGetGroupEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateGetGroupEq")
	}
	tplQuery[queryKindUpdEq], err = template.New("updeq").Parse(templateUpdEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateUpdEq")
	}
	tplQuery[queryKindDelEq], err = template.New("deleq").Parse(templateDelEq)
	if err != nil {
		return kerrors.WithMsg(err, "Failed to parse template templateDelEq")
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
		Generator: "go generate forge model",
		Version:   version,
		Package:   env.GoPackage,
	}
	if err := tplmain.Execute(fwriter, tplData); err != nil {
		return kerrors.WithMsg(err, "Failed to execute main model template")
	}

	for _, i := range modelDefs {
		mctx := klog.CtxWithAttrs(ctx, klog.AString("model", i.Ident))
		l.Debug(mctx, "Detected model", klog.AAny("fields", i.Fields))

		tplData := modelTemplateData{
			Prefix:     i.Prefix,
			ModelIdent: i.Ident,
			SQL:        i.genModelSQL(),
		}
		if err := tplmodel.Execute(fwriter, tplData); err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to execute model template for struct: %s", i.Ident))
		}
		for _, j := range queryGroupDefs[i.Prefix] {
			qctx := klog.CtxWithAttrs(mctx, klog.AString("query", j.Ident))
			l.Debug(qctx, "Detected query", klog.AAny("fields", j.Fields))

			querySQLStrings := j.genQuerySQL()
			numFields := len(j.Fields)
			for _, k := range j.Queries {
				tplData := queryTemplateData{
					Prefix:     i.Prefix,
					ModelIdent: j.Ident,
					Name:       k.Name,
					SQL:        querySQLStrings,
				}
				switch k.Kind {
				case queryKindGetOneEq:
					tplData.SQLCond = k.genQueryCondSQL(0)
				case queryKindGetGroup:
					tplData.SQLOrder = k.genQueryOrderSQL()
				case queryKindGetGroupEq:
					tplData.SQLCond = k.genQueryCondSQL(2)
					tplData.SQLOrder = k.genQueryOrderSQL()
				case queryKindUpdEq:
					tplData.SQLCond = k.genQueryCondSQL(numFields)
				case queryKindDelEq:
					tplData.SQLCond = k.genQueryCondSQL(0)
				}
				if err := tplQuery[k.Kind].Execute(fwriter, tplData); err != nil {
					return kerrors.WithMsg(err, fmt.Sprintf("Failed to execute template for query kind %s on struct %s of model %s", k.Kind, tplData.ModelIdent, tplData.Prefix))
				}
			}
		}
	}

	if err := fwriter.Flush(); err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to write to file: %s", opts.Output))
	}

	l.Info(ctx, "Generated model file", klog.AString("output", opts.Output))
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
	for _, i := range m.Constraints {
		fields := make([]string, 0, len(i.Columns))
		for _, j := range i.Columns {
			fields = append(fields, j.DBName)
		}
		sqlDefs = append(sqlDefs, fmt.Sprintf("%s (%s)", i.Kind, strings.Join(fields, ", ")))
	}
	if m.opts.Setup != "" {
		sqlDefs = append(sqlDefs, m.opts.Setup)
	}

	sqlIndicies := make([]modelIndex, 0, len(m.Indicies))
	for _, i := range m.Indicies {
		k := make([]string, 0, len(i.Columns))
		for _, j := range i.Columns {
			if j.Dir == "" {
				k = append(k, j.Field.DBName)
			} else {
				k = append(k, fmt.Sprintf("%s %s", j.Field.DBName, j.Dir))
			}
		}
		sqlIndicies = append(sqlIndicies, modelIndex{
			Name:    i.Name,
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

func (q *queryGroupDef) genQuerySQL() querySQLStrings {
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
		NumDBNames:   len(sqlDBNames),
		Idents:       strings.Join(sqlIdents, ", "),
		IdentRefs:    strings.Join(sqlIdentRefs, ", "),
		Placeholders: strings.Join(sqlPlaceholders, ", "),
		ColNum:       fmt.Sprintf("%d", colNum),
	}
}

func (q *queryDef) genQueryCondSQL(offset int) queryCondSQLStrings {
	sqlIdentParams := make([]string, 0, len(q.Conds))
	sqlDBCond := make([]string, 0, len(q.Conds))
	sqlIdentArgs := make([]string, 0, len(q.Conds))
	sqlArrIdentArgs := make([]string, 0, len(q.Conds))
	sqlArrIdentArgsLen := make([]string, 0, len(q.Conds))
	paramCount := offset
	for _, i := range q.Conds {
		paramName := strings.ToLower(i.Field.Ident)
		dbName := i.Field.DBName
		paramType := i.Field.GoType
		condText := "="
		switch i.Kind {
		case condNeq:
			condText = "<>"
		case condLt:
			condText = "<"
		case condLeq:
			condText = "<="
		case condGt:
			condText = ">"
		case condGeq:
			condText = ">="
		case condIn:
			paramName = paramName + "s"
			paramType = "[]" + paramType
		case condLike:
			paramName = paramName + "Prefix"
			condText = "LIKE"
		}

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
		IdentParams:     strings.Join(sqlIdentParams, ", "),
		DBCond:          strings.Join(sqlDBCond, " AND "),
		IdentArgs:       strings.Join(sqlIdentArgs, ", "),
		ArrIdentArgs:    sqlArrIdentArgs,
		ArrIdentArgsLen: strings.Join(sqlArrIdentArgsLen, "+"),
		ParamCount:      paramCount,
	}
}

func (q *queryDef) genQueryOrderSQL() queryOrderSQLStrings {
	colOrder := make([]string, 0, len(q.Order))
	for _, i := range q.Order {
		if i.Dir == "" {
			colOrder = append(colOrder, i.Field.DBName)
		} else {
			colOrder = append(colOrder, fmt.Sprintf("%s %s", i.Field.DBName, i.Dir))
		}
	}
	return queryOrderSQLStrings{
		DBOrder: strings.Join(colOrder, ", "),
	}
}

func parseModelDefinitions(modelObjects []dirObjPair, modelTag string, fset *token.FileSet, schema modelSchema) ([]modelDef, error) {
	modelDefs := make([]modelDef, 0, len(modelObjects))

	for _, i := range modelObjects {
		prefix := i.Dir.Directive
		if prefix == "" {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Model directive without prefix")
		}
		opts := schema.Models[prefix]
		if i.Obj.Kind != gopackages.ObjKindDeclType {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Model directive used on non-type declaration")
		}
		typeSpec, ok := i.Obj.Obj.(*ast.TypeSpec)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Unexpected directive object type")
		}
		structName := typeSpec.Name.Name
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Model directive used on non-struct type declaration")
		}
		if structType.Incomplete {
			return nil, kerrors.WithMsg(nil, "Unexpected incomplete struct definition")
		}
		astFields, err := findFields(modelTag, structType, fset)
		if err != nil {
			return nil, err
		}
		if len(astFields) == 0 {
			return nil, kerrors.WithKind(nil, ErrInvalidModel, "No model fields found on struct")
		}
		modelFields, fieldMap, err := parseModelFields(astFields)
		if err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Invalid model fields for struct %s", structName))
		}
		constraints := make([]modelConstraint, 0, len(opts.Model.Constraints))
		for _, i := range opts.Model.Constraints {
			if i.Kind == "" {
				return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Missing constraint kind for struct %s", structName))
			}
			if len(i.Columns) == 0 {
				return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("No columns for constraint of struct %s", structName))
			}
			fields := make([]modelField, 0, len(i.Columns))
			for _, j := range i.Columns {
				f, ok := fieldMap[j]
				if !ok {
					return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Unknown field %s for constraint of struct %s", j, structName))
				}
				fields = append(fields, f)
			}
			constraints = append(constraints, modelConstraint{
				Kind:    i.Kind,
				Columns: fields,
			})
		}
		indicies := make([]modelIndexDef, 0, len(opts.Model.Indicies))
		for _, i := range opts.Model.Indicies {
			if i.Name == "" {
				return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("No name for index of struct %s", structName))
			}
			if len(i.Columns) == 0 {
				return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("No columns for index of struct %s", structName))
			}
			columns := make([]modelIndexColumn, 0, len(i.Columns))
			for _, j := range i.Columns {
				f, ok := fieldMap[j.Col]
				if !ok {
					return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Unknown field %s for index of struct %s", j, structName))
				}
				columns = append(columns, modelIndexColumn{
					Field: f,
					Dir:   j.Dir,
				})
			}
			indicies = append(indicies, modelIndexDef{
				Name:    i.Name,
				Columns: columns,
			})
		}
		modelDefs = append(modelDefs, modelDef{
			Prefix:      prefix,
			Ident:       structName,
			Fields:      modelFields,
			Constraints: constraints,
			Indicies:    indicies,
			opts:        opts.Model,
			fieldMap:    fieldMap,
		})
	}

	return modelDefs, nil
}

func parseModelFields(astfields []astField) ([]modelField, map[string]modelField, error) {
	fields := make([]modelField, 0, len(astfields))
	seenFields := map[string]modelField{}
	for n, i := range astfields {
		dbName, dbType, ok := strings.Cut(i.Tags, ",")
		if !ok || dbName == "" || dbType == "" {
			return nil, nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Model field tag must be dbname,dbtype on field %s", i.Ident))
		}
		if dup, ok := seenFields[dbName]; ok {
			return nil, nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Duplicate field %s on %s and %s", dbName, i.Ident, dup.Ident))
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
	}
	return fields, seenFields, nil
}

func parseQueryDefinitions(queryObjects []dirObjPair, modelTag string, modelDefs map[string]modelDef, fset *token.FileSet, schema modelSchema) (map[string][]queryGroupDef, error) {
	queryGroupDefs := map[string][]queryGroupDef{}

	for _, i := range queryObjects {
		prefix := i.Dir.Directive
		if prefix == "" {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Query directive without prefix")
		}
		mdef, ok := modelDefs[prefix]
		if !ok {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, fmt.Sprintf("Query directive prefix %s without model definition", prefix))
		}
		if i.Obj.Kind != gopackages.ObjKindDeclType {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Query directive used on non-type declaration")
		}
		typeSpec, ok := i.Obj.Obj.(*ast.TypeSpec)
		if !ok {
			return nil, kerrors.WithMsg(nil, "Unexpected directive object type")
		}
		structName := typeSpec.Name.Name
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Query directive used on non-struct type declaration")
		}
		if structType.Incomplete {
			return nil, kerrors.WithMsg(nil, "Unexpected incomplete struct definition")
		}
		astFields, err := findFields(modelTag, structType, fset)
		if err != nil {
			return nil, err
		}
		if len(astFields) == 0 {
			return nil, kerrors.WithKind(nil, ErrInvalidModel, "No query fields found on struct")
		}
		fields, err := parseQueryFields(astFields, mdef.fieldMap)
		if err != nil {
			return nil, kerrors.WithMsg(err, fmt.Sprintf("Invalid query fields for struct %s", structName))
		}
		opts := schema.Models[prefix].Queries[structName]
		if len(opts) == 0 {
			return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Query struct %s missing queries", structName))
		}
		queries := make([]queryDef, 0, len(opts))
		for _, j := range opts {
			kind, err := parseQueryKind(j.Kind)
			if err != nil {
				return nil, kerrors.WithMsg(err, fmt.Sprintf("Invalid query kind for %s on struct %s", j.Name, structName))
			}
			if j.Name == "" {
				return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Query name missing for kind %s on struct %s", j.Kind, structName))
			}
			def := queryDef{
				Kind: kind,
				Name: j.Name,
			}
			switch kind {
			case queryKindGetOneEq, queryKindGetGroupEq, queryKindUpdEq, queryKindDelEq:
				{
					if len(j.Conditions) == 0 {
						return nil, kerrors.WithKind(err, ErrInvalidModel, fmt.Sprintf("Query missing condition fields for %s %s on struct %s", j.Kind, j.Name, structName))
					}
					k := make([]queryCondField, 0, len(j.Conditions))
					for _, c := range j.Conditions {
						field, ok := mdef.fieldMap[c.Col]
						if !ok {
							return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Unknown condition field %s for %s %s on struct %s", c.Col, j.Kind, j.Name, structName))
						}
						cond, err := parseCond(c.Cond)
						if err != nil {
							return nil, kerrors.WithMsg(err, fmt.Sprintf("Invalid condition for field %s on query %s %s of struct %s", c.Col, j.Kind, j.Name, structName))
						}
						k = append(k, queryCondField{
							Kind:  cond,
							Field: field,
						})
					}
					def.Conds = k
				}
			default:
				if len(j.Conditions) != 0 {
					return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Query kind %s does not take conditions on %s of struct %s", j.Kind, j.Name, structName))
				}
			}
			switch kind {
			case queryKindGetGroup, queryKindGetGroupEq:
				{
					k := make([]queryOrderField, 0, len(j.Order))
					for _, c := range j.Order {
						field, ok := mdef.fieldMap[c.Col]
						if !ok {
							return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Unknown order field %s for %s %s on struct %s", c.Col, j.Kind, j.Name, structName))
						}
						k = append(k, queryOrderField{
							Field: field,
							Dir:   c.Dir,
						})
					}
					def.Order = k
				}
			default:
				if len(j.Order) != 0 {
					return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Query kind %s does not take order on %s of struct %s", j.Kind, j.Name, structName))
				}
			}
			queries = append(queries, def)
		}
		queryGroupDefs[prefix] = append(queryGroupDefs[prefix], queryGroupDef{
			Ident:   structName,
			Fields:  fields,
			Queries: queries,
		})
	}

	return queryGroupDefs, nil
}

func parseQueryFields(astfields []astField, fieldMap map[string]modelField) ([]queryField, error) {
	var fields []queryField
	for n, i := range astfields {
		dbName, _, _ := strings.Cut(i.Tags, ",")
		if dbName == "" {
			return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Query field opt must be dbname for field %s", i.Ident))
		}
		if mfield, ok := fieldMap[dbName]; !ok || i.GoType != mfield.GoType {
			return nil, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Field %s with type %s does not exist on model", dbName, i.GoType))
		}
		f := queryField{
			Ident:  i.Ident,
			GoType: i.GoType,
			DBName: dbName,
			Num:    n + 1,
		}
		fields = append(fields, f)
	}
	return fields, nil
}

type (
	queryKind int
)

const (
	queryKindUnknown queryKind = iota
	queryKindGetOneEq
	queryKindGetGroup
	queryKindGetGroupEq
	queryKindUpdEq
	queryKindDelEq
)

func parseQueryKind(kind string) (queryKind, error) {
	switch kind {
	case "getoneeq":
		return queryKindGetOneEq, nil
	case "getgroup":
		return queryKindGetGroup, nil
	case "getgroupeq":
		return queryKindGetGroupEq, nil
	case "updeq":
		return queryKindUpdEq, nil
	case "deleq":
		return queryKindDelEq, nil
	default:
		return queryKindUnknown, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Illegal query kind %s", kind))
	}
}

func (q queryKind) String() string {
	switch q {
	case queryKindGetOneEq:
		return "getoneeq"
	case queryKindGetGroup:
		return "getgroup"
	case queryKindGetGroupEq:
		return "getgroupeq"
	case queryKindUpdEq:
		return "updeq"
	case queryKindDelEq:
		return "deleq"
	default:
		return "unknown"
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

func parseCond(cond string) (condType, error) {
	switch cond {
	case "", "eq":
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
		return condUnknown, kerrors.WithKind(nil, ErrInvalidModel, fmt.Sprintf("Illegal cond type %s", cond))
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
			return nil, kerrors.WithKind(nil, ErrInvalidFile, "Only one field allowed per tag")
		}

		ident := field.Names[0].Name

		var goType bytes.Buffer
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
