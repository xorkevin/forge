## forge validation

Generates validations

### Synopsis

Generates common validation on go structs

forge validation is called with the following environment variables:

	GOPACKAGE: name of the go package
	GOFILE: name of the go source file

forge validation code generates a validation method for structs, where for
every struct field tagged with valid, a function based on the tag value will be
called.

For example, for a struct defined as:

	type test struct {
		field1 string `valid:"field"`
		field2 int `valid:"other"`
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
		field1 string `valid:"field"`
		field2 int `valid:"other,has"`
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
		field1 string `valid:"field"`
		field2 int `valid:"other,opt"`
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



```
forge validation [query ...] [flags]
```

### Options

```
      --directive string      comment directive of types that should be validated (default "forge:valid")
      --field-tag string      go struct field tag for defining validations (default "valid")
      --has-prefix string     prefix of has functions (default "validhas")
  -h, --help                  help for validation
      --ignore string         regex for filenames of files that should be ignored
      --include string        regex for filenames of files that should be included
      --opt-prefix string     prefix of opt functions (default "validopt")
  -o, --output string         output filename (default "validation_gen.go")
  -p, --prefix string         prefix of identifiers in generated file (default "valid")
      --valid-prefix string   prefix of validation functions (default "valid")
  -v, --verbose               increase the verbosity of output
```

### Options inherited from parent commands

```
      --config string   config file (default is $XDG_CONFIG_HOME/.forge.yaml)
      --debug           turn on debug output
```

### SEE ALSO

* [forge](forge.md)	 - A code generation utility

