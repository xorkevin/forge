package cmd

import (
	"github.com/hackform/forge/gen"
	"github.com/spf13/cobra"
)

var (
	genNoIgnore bool
	genPrefix   string
	genDryRun   bool
	genVerbose  bool
)

// genCmd represents the model command
var genCmd = &cobra.Command{
	Use:   "gen [path | file glob ...]",
	Short: "Executes command line directives in source files",
	Long: `Executes command line directives in source files

Directives appear in the form of:

	<prefix>forge:gen command args`,
	Run: func(cmd *cobra.Command, args []string) {
		gen.Execute(genNoIgnore, genPrefix, genDryRun, genVerbose, args)
	},
}

func init() {
	rootCmd.AddCommand(genCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// genCmd.PersistentFlags().String("foo", "", "A help for foo")
	genCmd.PersistentFlags().BoolVarP(&genNoIgnore, "noignore", "i", false, "do not use .gitignore")
	genCmd.PersistentFlags().StringVarP(&genPrefix, "prefix", "p", "+", "set prefix for forge directive")
	genCmd.PersistentFlags().BoolVarP(&genDryRun, "dryrun", "n", false, "do not exec directives but print what would be executed")
	genCmd.PersistentFlags().BoolVarP(&genVerbose, "verbose", "v", false, "increase the verbosity of the output")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// genCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
