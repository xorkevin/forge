package cmd

import (
	"github.com/spf13/cobra"
	"xorkevin.dev/forge/model"
)

var (
	modelVerbose      bool
	modelOutputFile   string
	modelOutputPrefix string
	modelTableName    string
	modelModelName    string
)

// modelCmd represents the model command
var modelCmd = &cobra.Command{
	Use:   "model [query ...]",
	Short: "Generates models",
	Long: `Generates common SQL patterns needed for relational models

forge model is called with the following environment variables:

	GOPACKAGE: name of the go package
	GOFILE: name of the go source file

forge model code generates go functions for SQL select, insert, update, and
delete for a model by default, with additional queries provided as arguments.

The SQL table's columns for a model are specified by the "model" tag on fields
of a Go struct representing a row of the table. A "model" tag's value has the
following syntax:

	column_name,sql_type

Fields without a "model" tag are ignored.

A query allows additional common case select statements to be code generated.
It is specified by a "query" tag on a struct representing a row of the query
result with a value of the syntax:

	column_name[,flag[,args ...][; ...]]

column_name refers to the column name defined in the model. The go field type
must also be the same between the model and the query.

Fields without a "query" tag are ignored.

Valid flags are:
	- getoneeq: args(equal_field,...), gets a single row where the equal field(s)
	are equal to the input
	- getgroup: (no args), gets all rows ordered by the field value
	- getgroupeq: args(equal_field,...), gets all rows where the equal field(s)
	are equal to the input ordered by the field value
	- updeq: args(equal_field,...), updates all rows where the equal fields(s)
	are equal to the input
	- deleq: args(equal_field,...), deletes all rows where the equal fields(s)
	are equal to the input

equal_field by default has a condition of eq, but it may be explicitly
specified by column_name|cond. cond may be one of:
	- eq: column value equals the input
	- neq: column value not equal to the input
	- lt: column value less than the input
	- leq: column value less than or equal to the input
	- gt: column value greater than the input
	- geq: column value greater than or equal to the input
	- arr: column value equals one of the values of the input set
	- like: column value like the input

`,
	Run: func(cmd *cobra.Command, args []string) {
		model.Execute(modelVerbose, versionString, modelOutputFile, modelOutputPrefix, modelTableName, modelModelName, args)
	},
}

func init() {
	rootCmd.AddCommand(modelCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// modelCmd.PersistentFlags().String("foo", "", "A help for foo")
	modelCmd.PersistentFlags().StringVarP(&modelOutputFile, "output", "o", "model_gen.go", "output filename")
	modelCmd.PersistentFlags().StringVarP(&modelOutputPrefix, "prefix", "p", "", "prefix of identifiers in generated file")
	modelCmd.MarkFlagRequired("prefix")
	modelCmd.PersistentFlags().StringVarP(&modelTableName, "table", "t", "", "name of the table in the database")
	modelCmd.MarkFlagRequired("table")
	modelCmd.PersistentFlags().StringVarP(&modelModelName, "model", "m", "", "name of the model identifier")
	modelCmd.MarkFlagRequired("model")
	modelCmd.PersistentFlags().BoolVarP(&modelVerbose, "verbose", "v", false, "increase the verbosity of output")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// modelCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
