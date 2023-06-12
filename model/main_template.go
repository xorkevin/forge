package model

const templateMain = `// Code generated by {{.Generator}} {{.Version}}; DO NOT EDIT.

package {{.Package}}

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"xorkevin.dev/forge/model/sqldb"
)
`
