package orm

import (
	"context"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type ctxKey struct {
	name string
}

var txCtxKey = &ctxKey{name: "tx"}

type DB struct {
	*gorm.DB
}

type DBOption func(*gorm.Config)

func WithLogger(l gormlogger.Interface) DBOption {
	return func(cfg *gorm.Config) {
		cfg.Logger = l
	}
}

func NewDB(conn gorm.ConnPool, driver string, opts ...DBOption) (*DB, error) {
	var dialector gorm.Dialector

	switch driver {
	case "postgres":
		dialector = postgres.New(postgres.Config{
			Conn: conn,
		})
	case "mysql":
		dialector = mysql.New(mysql.Config{
			Conn: conn,
		})
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	gormCfg := &gorm.Config{}
	for _, opt := range opts {
		opt(gormCfg)
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) WithContext(ctx context.Context) *gorm.DB {
	v := ctx.Value(txCtxKey)
	if v != nil {
		if tx, ok := v.(*gorm.DB); ok {
			return tx
		}
	}

	return db.DB.WithContext(ctx)
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
