package orm

import (
	"context"
	"database/sql"
	"fmt"

	ctxkey "gss/pkg/ctxutil/key"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

func NewDB(sqlDB *sql.DB, driver string) (*DB, error) {
	var dialector gorm.Dialector

	switch driver {
	case "postgres":
		dialector = postgres.New(postgres.Config{
			Conn: sqlDB,
		})
	case "mysql":
		dialector = mysql.New(mysql.Config{
			Conn: sqlDB,
		})
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) WithContext(ctx context.Context) *gorm.DB {
	v := ctx.Value(ctxkey.TransactionContextKey)
	if v != nil {
		if tx, ok := v.(*gorm.DB); ok {
			return tx
		}
	}

	return db.WithContext(ctx)
}
