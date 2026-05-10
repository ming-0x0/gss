package orm

import (
	"context"
	"database/sql"
	"fmt"

	ctxkey "gss/pkg/ctxutil/key"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type DB struct {
	db *bun.DB
}

func NewDB(sqlDB *sql.DB, driver string) (*DB, error) {
	switch driver {
	case "postgres":
		return &DB{
			db: bun.NewDB(sqlDB, pgdialect.New()),
		}, nil
	case "mysql":
		return &DB{
			db: bun.NewDB(sqlDB, mysqldialect.New()),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func (db *DB) WithContext(ctx context.Context) bun.IDB {
	v := ctx.Value(ctxkey.TransactionContextKey)
	if v != nil {
		if tx, ok := v.(bun.Tx); ok {
			return tx
		}
	}

	return db.db
}

func (db *DB) AddQueryHook(hook bun.QueryHook) {
	db.db.AddQueryHook(hook)
}
