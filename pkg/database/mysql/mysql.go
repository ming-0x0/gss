package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type config struct {
	dsn             string
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
}

func WithDSN(host string, port int, user string, password string, dbName string) option {
	return func(cfg *config) {
		cfg.dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
			user, password, host, port, dbName)
	}
}

func WithMaxOpenConns(maxOpenConns int) option {
	return func(cfg *config) {
		cfg.maxOpenConns = maxOpenConns
	}
}

func WithMaxIdleConns(maxIdleConns int) option {
	return func(cfg *config) {
		cfg.maxIdleConns = maxIdleConns
	}
}

func WithConnMaxLifetime(connMaxLifetime time.Duration) option {
	return func(cfg *config) {
		cfg.connMaxLifetime = connMaxLifetime
	}
}

func WithConnMaxIdleTime(connMaxIdleTime time.Duration) option {
	return func(cfg *config) {
		cfg.connMaxIdleTime = connMaxIdleTime
	}
}

type option func(*config)

func Open(opts ...option) (*sql.DB, error) {
	cfg := &config{
		maxOpenConns:    10,
		maxIdleConns:    10,
		connMaxLifetime: 300,
		connMaxIdleTime: 60,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// open the database connection pool
	db, err := sql.Open("mysql", cfg.dsn)
	if err != nil {
		return nil, err
	}

	// set the connection pool configuration
	db.SetMaxOpenConns(cfg.maxOpenConns)
	db.SetMaxIdleConns(cfg.maxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.connMaxLifetime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(cfg.connMaxIdleTime) * time.Second)

	// ping the database
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func Close(db *sql.DB) error {
	return db.Close()
}
