package sqldb

import (
	"context"
	"database/sql"
)

type (
	// Executor is the interface of the subset of methods shared by [sql.DB] and [sql.Tx]
	Executor interface {
		ExecContext(ctx context.Context, query string, args ...interface{}) (Result, error)
		QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error)
		QueryRowContext(ctx context.Context, query string, args ...interface{}) Row
	}

	// Result is [sql.Result]
	Result = sql.Result

	// SQLRows is the interface boundary of [sql.Rows]
	Rows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Err() error
		Close() error
	}

	// Row is the interface boundary of [sql.Row]
	Row interface {
		Scan(dest ...interface{}) error
		Err() error
	}
)
