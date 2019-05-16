package cmd

import (
	"github.com/hackform/forge/model"
	"github.com/spf13/cobra"
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
	Use:   "model",
	Short: "Generates models",
	Long:  `Generates common SQL patterns needed for relational models`,
	Run: func(cmd *cobra.Command, args []string) {
		model.Execute(modelVerbose, modelOutputFile, modelOutputPrefix, modelTableName, modelModelName, args)
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
