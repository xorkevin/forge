package cmd

import (
	"github.com/spf13/cobra"
	"xorkevin.dev/forge/validation"
	"xorkevin.dev/klog"
)

type (
	validFlags struct {
		opts validation.Opts
	}
)

func (c *Cmd) getValidationCmd() *cobra.Command {
	validationCmd := &cobra.Command{
		Use:   "validation [query ...]",
		Short: "Generates validations",
		Long: `Generates common validation on go structs

forge validation is called with the following environment variables:

	GOPACKAGE: name of the go package
	GOFILE: name of the go source file

forge validation code generates a validation method for structs, where for
every struct field tagged with valid, a function based on the tag value will be
called.

For example, for a struct defined as:

	type test struct {
		field1 string ` + "`" + `valid:"field"` + "`" + `
		field2 int ` + "`" + `valid:"other"` + "`" + `
	}

a method will be generated with the name of prefix (default: valid) calling
functions beginning with valid-prefix (default: valid) or has-prefix (default:
validhas) and returning error. The example from above with the default options
would generate:

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

A valid tag value may also be suffixed with ",opt" as in:

	type test struct {
		field1 string ` + "`" + `valid:"field"` + "`" + `
		field2 int ` + "`" + `valid:"other,opt"` + "`" + `
	}

which with the default options would generate:

	func (r test) valid() error {
		if err := validField(r.field1); err != nil {
			return err
		}
		if err := validoptOther(r.field2); err != nil {
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

The "opt" suffix is a feature designed to allow certain fields to be omitted.

`,
		Run:               c.execValidation,
		DisableAutoGenTag: true,
	}
	validationCmd.PersistentFlags().StringVarP(&c.validFlags.opts.Output, "output", "o", "validation_gen.go", "output filename")
	validationCmd.PersistentFlags().StringVarP(&c.validFlags.opts.Prefix, "prefix", "p", "valid", "prefix of identifiers in generated file")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.PrefixValid, "valid-prefix", "valid", "prefix of validation functions")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.PrefixHas, "has-prefix", "validhas", "prefix of has functions")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.PrefixOpt, "opt-prefix", "validopt", "prefix of opt functions")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.Include, "include", "", "regex for filenames of files that should be included")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.Ignore, "ignore", "", "regex for filenames of files that should be ignored")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.Directive, "directive", "forge:valid", "comment directive of types that should be validated")
	validationCmd.PersistentFlags().StringVar(&c.validFlags.opts.Tag, "field-tag", "valid", "go struct field tag for defining validations")
	return validationCmd
}

func (c *Cmd) execValidation(cmd *cobra.Command, args []string) {
	if err := validation.Execute(
		c.log.Logger.Sublogger("", klog.AString("cmd", "validation")),
		c.version,
		c.validFlags.opts,
	); err != nil {
		c.logFatal(err)
		return
	}
}
