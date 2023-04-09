## forge model

Generates models

### Synopsis

Generates common SQL patterns needed for relational models

forge model is called with the following environment variables:

    GOPACKAGE: name of the go package
    GOFILE: name of the go source file

forge model code generates go functions for SQL select, insert, update, and
delete for a model by default, with additional queries provided as arguments.

The SQL table's columns for a model are specified by the "model" tag on fields
of a Go struct representing a row of the table. A "model" tag's value has the
following syntax:

    column_name,sql_type[;opt[,args ...][; ...]]

Fields without a "model" tag are ignored.

Valid opts are:

- index: args(field,...), creates a index from the provided fields and
  current field

A query allows additional common case select statements to be code generated.
It is specified by a "query" tag on a struct representing a row of the query
result with a value of the syntax:

    column_name[;flag[,args ...][; ...]]

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
- in: column value equals one of the values of the input set
- like: column value like the input


```
forge model [query ...] [flags]
```

### Options

```
  -h, --help                     help for model
      --ignore string            regex for filenames of files that should be ignored
      --include string           regex for filenames of files that should be included
      --model-directive string   comment directive of types that are models (default "forge:model")
      --model-tag string         go struct tag for defining model fields (default "model")
  -o, --output string            output filename (default "model_gen.go")
      --query-directive string   comment directive of types that are model queries (default "forge:model:query")
      --query-tag string         go struct tag for defining query fields (default "query")
```

### Options inherited from parent commands

```
      --log-json           output json logs
      --log-level string   log level (default "info")
```

### SEE ALSO

* [forge](forge.md)	 - A code generation utility

