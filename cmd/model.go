package cmd

import (
	"github.com/spf13/cobra"
	"xorkevin.dev/forge/model"
	"xorkevin.dev/klog"
)

type (
	modelFlags struct {
		opts model.Opts
	}
)

func (c *Cmd) getModelCmd() *cobra.Command {
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Generates models",
		Long: `Generates common SQL patterns needed for relational models

forge model is called with the following environment variables:

    GOPACKAGE: name of the go package
    GOFILE: name of the go source file

forge model code generates go functions for SQL select, insert, update, and
delete for a model. Only structs with the following comment directive are
considered:

    //forge:model modelPrefix
    Model struct {}

The SQL table's columns for a model are specified by the "model" tag on fields
of a Go struct representing a row of the table. A "model" tag's value has the
following syntax:

    column_name,sql_type

Fields without a "model" tag are ignored.

A query allows additional statements to be code generated. It is specified by a
"model" tag on a struct which represents a column of the query result and has
the following syntax:

    column_name[,sql_type]

column_name refers to the column name defined in the model. The go field type
must also be the same between the model and the query. sql_type is optional and
ignored for queries. Likewise, fields without a "model" tag are ignored.

A separate schema file (model.json by default) is used to specify additional
constraints, conditions, and queries. The schema is as follows:

    {
      "modelPrefix": {
        "model": {
          "setup": "optional text appended to the end of the model setup query",
          "constraints": [
            {"kind": "PRIMARY KEY/UNIQUE/etc.", "columns": ["col1", "etc"]}
          ],
          "indicies": [
            {"columns": ["col1", "etc"]}
          ]
        },
        "queries": {
          "StructName": [
            {
              "kind": "getoneeq/getgroup/etc.",
              "name": "QueryName",
              "conditions": [
                {"col": "col1", "cond": "eq (default)/neq/etc."}
              ],
              "order": [
                {"col": "col1", "dir": "empty/ASC/DESC/etc."}
              ]
            }
          ]
        }
      }
    }

Valid query kinds are:

- getoneeq: gets a single row where the equal field(s) are equal to the input
- getgroup: gets all rows
- getgroupeq: gets all rows where the field(s) are equal to the input
- updeq: updates all rows where the fields(s) are equal to the input
- deleq: deletes all rows where the fields(s) are equal to the input

field by default has a condition of eq, but it may be explicitly specified.
cond may be one of:

- eq: column value equals the input
- neq: column value not equal to the input
- lt: column value less than the input
- leq: column value less than or equal to the input
- gt: column value greater than the input
- geq: column value greater than or equal to the input
- in: column value equals one of the values of the input set
- like: column value like the input
`,
		Run:               c.execModel,
		DisableAutoGenTag: true,
	}
	modelCmd.PersistentFlags().StringVarP(&c.modelFlags.opts.Output, "output", "o", "model_gen.go", "output filename")
	modelCmd.PersistentFlags().StringVarP(&c.modelFlags.opts.Schema, "schema", "s", "model.json", "model schema")
	modelCmd.PersistentFlags().StringVar(&c.modelFlags.opts.Include, "include", "", "regex for filenames of files that should be included")
	modelCmd.PersistentFlags().StringVar(&c.modelFlags.opts.Ignore, "ignore", "", "regex for filenames of files that should be ignored")
	modelCmd.PersistentFlags().StringVar(&c.modelFlags.opts.ModelDirective, "model-directive", "forge:model", "comment directive of types that are models")
	modelCmd.PersistentFlags().StringVar(&c.modelFlags.opts.QueryDirective, "query-directive", "forge:model:query", "comment directive of types that are model queries")
	modelCmd.PersistentFlags().StringVar(&c.modelFlags.opts.ModelTag, "model-tag", "model", "go struct tag for defining model fields")
	return modelCmd
}

func (c *Cmd) execModel(cmd *cobra.Command, args []string) {
	log := c.log.Logger.Sublogger("", klog.AString("cmd", "model"))
	if err := model.Execute(
		log,
		c.version,
		c.modelFlags.opts,
	); err != nil {
		c.logFatal(err)
		return
	}
}
