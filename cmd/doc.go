package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// docCmd represents the doc command
var docCmd = &cobra.Command{
	Use:   "doc [format]",
	Short: "generate documentation for forge",
	Long: `generate documentation for forge in several formats

  valid formats are:
    * man (default)
    * markdown, md`,
	Args:      cobra.OnlyValidArgs,
	ValidArgs: []string{"man", "md", "markdown"},
	Run: func(cmd *cobra.Command, args []string) {
		docFormat := "man"
		if len(args) > 0 {
			switch args[0] {
			case "man":
				docFormat = "man"
			case "md", "markdown":
				docFormat = "md"
			}
		}
		if docFormat == "man" {
			if err := doc.GenManTree(rootCmd, &doc.GenManHeader{
				Title:   "forge",
				Section: "1",
			}, "./doc"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		} else if docFormat == "md" {
			if err := doc.GenMarkdownTree(rootCmd, "./doc"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(docCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// docCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// docCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
