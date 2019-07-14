package cmd

import (
	"github.com/hackform/forge/validation"
	"github.com/spf13/cobra"
)

var (
	validationVerbose        bool
	validationOutputFile     string
	validationOutputPrefix   string
	validationValidatePrefix string
	validationHasPrefix      string
)

// validationCmd represents the validation command
var validationCmd = &cobra.Command{
	Use:   "validation [query ...]",
	Short: "Generates validations",
	Long:  `Generates common validation on go structs`,
	Run: func(cmd *cobra.Command, args []string) {
		validation.Execute(validationVerbose, validationOutputFile, validationOutputPrefix, validationValidatePrefix, validationHasPrefix, args)
	},
}

func init() {
	rootCmd.AddCommand(validationCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// validationCmd.PersistentFlags().String("foo", "", "A help for foo")
	validationCmd.PersistentFlags().StringVarP(&validationOutputFile, "output", "o", "validation_gen.go", "output filename")
	validationCmd.PersistentFlags().StringVarP(&validationOutputPrefix, "prefix", "p", "", "prefix of identifiers in generated file")
	validationCmd.MarkFlagRequired("prefix")
	validationCmd.PersistentFlags().StringVarP(&validationValidatePrefix, "validatep", "c", "validate", "prefix of validation functions")
	validationCmd.PersistentFlags().StringVarP(&validationHasPrefix, "hasp", "d", "validhas", "prefix of has functions")
	validationCmd.PersistentFlags().BoolVarP(&validationVerbose, "verbose", "v", false, "increase the verbosity of output")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// validationCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
