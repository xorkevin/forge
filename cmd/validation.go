package cmd

import (
	"github.com/spf13/cobra"
	"xorkevin.dev/forge/validation"
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
	Long: `Generates common validation on go structs

forge validation code generates a validation method for structs, where for
every struct field tagged with valid, a function based on the tag value will be
called.

For example, for a struct defined as:

	type test struct {
		field1 string ` + "`" + `valid:"field"` + "`" + `
		field2 int ` + "`" + `valid:"other"` + "`" + `
	}

a method will be generated with the name of prefix (default: valid) calling
functions beginning with validatep (default: valid) or hasp (default: validhas)
and returning error. The example from above with the default options would
generate:

	func (r test) valid() error {
		if err := validField(r.field1); err != nil {
			return err
		}
		if err := validOther(r.field2); err != nil {
			return err
		}
		return nil
	}

A valid tag value may also be suffixed with ",has" as in:

	type test struct {
		field1 string ` + "`" + `valid:"field"` + "`" + `
		field2 int ` + "`" + `valid:"other,has"` + "`" + `
	}

which with the default options would generate:

	func (r test) valid() error {
		if err := validField(r.field1); err != nil {
			return err
		}
		if err := validhasOther(r.field2); err != nil {
			return err
		}
		return nil
	}

The "has" suffix is a feature designed to allow certain fields to be validated
by functions that are less restrictive than the non-has variant. This is to
allow the robustness principle: "Be conservative in what you send, be liberal
in what you accept." The "has" suffix is also useful in cases where the legal
values of newly created fields may change over time, such as password length
requirements increasing, but the application must still allow older existing
input values.

`,
	Run: func(cmd *cobra.Command, args []string) {
		validation.Execute(validationVerbose, versionString, validationOutputFile, validationOutputPrefix, validationValidatePrefix, validationHasPrefix, args)
	},
}

func init() {
	rootCmd.AddCommand(validationCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// validationCmd.PersistentFlags().String("foo", "", "A help for foo")
	validationCmd.PersistentFlags().StringVarP(&validationOutputFile, "output", "o", "validation_gen.go", "output filename")
	validationCmd.PersistentFlags().StringVarP(&validationOutputPrefix, "prefix", "p", "valid", "prefix of identifiers in generated file")
	validationCmd.PersistentFlags().StringVarP(&validationValidatePrefix, "validatep", "c", "valid", "prefix of validation functions")
	validationCmd.PersistentFlags().StringVarP(&validationHasPrefix, "hasp", "d", "validhas", "prefix of has functions")
	validationCmd.PersistentFlags().BoolVarP(&validationVerbose, "verbose", "v", false, "increase the verbosity of output")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// validationCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
