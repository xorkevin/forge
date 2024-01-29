## forge model

Generates models

### Synopsis

Generates common SQL patterns needed for relational models

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


```
forge model [flags]
```

### Options

```
  -h, --help                        help for model
      --ignore string               regex for filenames of files that should be ignored
      --include string              regex for filenames of files that should be included
      --model-directive string      comment directive of types that are models (default "forge:model")
      --model-tag string            go struct tag for defining model fields (default "model")
  -o, --output string               output filename (default "model_gen.go")
      --placeholder-prefix string   query numeric placeholder prefix (default "$")
      --query-directive string      comment directive of types that are model queries (default "forge:model:query")
  -s, --schema string               model schema (default "model.json")
```

### Options inherited from parent commands

```
      --log-json           output json logs
      --log-level string   log level (default "info")
```

### SEE ALSO

* [forge](forge.md)	 - A code generation utility

