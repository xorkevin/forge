# forge

Code generation utility

install with `$ go get xorkevin.dev/forge`

#### Motivation

`forge` is a code generation tool designed to solve some metaprogramming tasks
in Go. It currently code generates PostgreSQL SQL functions that use the
`database/sql` and `github.com/lib/pq` packages. And it generates struct
validation methods. It will not solve all problems but it is designed to solve
the most common use cases, and reduce handwritten code duplication.

## Usage

Reference the `doc` directory or run `forge help`.
