package main

import (
	"fmt"
	"gss/configs"
	"gss/pkg/timezone"
	"time"
)

func main() {
	timezone.SetTimeZoneICT()

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	err := configs.Load()
	if err != nil {
		panic(err)
	}

	for {
		cfg := configs.Get() // lock-free read
		fmt.Printf("Port: %s | DB: %d\n", cfg.MySQL.Host, cfg.MySQL.Port)
		time.Sleep(3 * time.Second)
	}

	// logger := logger.New(
	// 	logger.WithLevel(cfg.Logger.Level),
	// )

	// logger.InfoContext(ctx, "Connecting to database...")
	// sqlDB, err := mysql.New(
	// 	mysql.WithDSN(cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Database),
	// 	mysql.WithMaxOpenConns(cfg.MySQL.MaxOpenConns),
	// 	mysql.WithMaxIdleConns(cfg.MySQL.MaxIdleConns),
	// 	mysql.WithConnMaxLifetime(cfg.MySQL.ConnMaxLifetime),
	// 	mysql.WithConnMaxIdleTime(cfg.MySQL.ConnMaxIdleTime),
	// )
	// if err != nil {
	// 	logger.FatalContext(ctx, "Failed to connect to database", "error", err)
	// }

	// logger.InfoContext(ctx, "Connected to database successfully.")
	// defer mysql.Close(sqlDB)
}
