package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	deliveryHTTP "gss/internal/delivery/http"
	"gss/internal/delivery/http/handler"
	"gss/internal/delivery/http/handler/auth"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm/bunutil"
	"gss/internal/repository"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Initialize logger
	l := logger.New(
		logger.WithLevel(getEnv("LOG_LEVEL", "info")),
	)

	// Initialize database
	driver := getEnv("DB_DRIVER", "mysql")
	dsn := getEnv("DB_DSN", "root:password@tcp(localhost:3306)/gss?parseTime=true")
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		l.Fatal("Failed to open database", "error", err.Error())
	}

	db, err := bunutil.NewDB(sqlDB, driver)
	if err != nil {
		l.Fatal("Failed to initialize bunutil DB", "error", err.Error())
	}

	// Add query hook for logging
	db.AddQueryHook(bunutil.New(
		bunutil.WithLogger(l),
		bunutil.WithLevel(getEnv("LOG_LEVEL", "info")),
	))

	// Initialize repository
	userRepo := repository.NewUserRepository(db, l)

	// Initialize handler
	baseHandler := handler.NewHandler(getEnv("LOG_LEVEL", "debug"))
	contextTimeout := 5 * time.Second
	authHandler := auth.NewHandler(baseHandler, userRepo, contextTimeout, l)

	// Initialize router
	version := getEnv("APP_VERSION", "1.0.0")
	router := deliveryHTTP.NewRouter(authHandler, version)

	// Start server
	port := getEnv("PORT", "8080")
	l.Info("Starting server", "port", port)

	if err := router.Run(fmt.Sprintf(":%s", port)); err != nil && err != http.ErrServerClosed {
		l.Fatal("Server failed to start", "error", err.Error())
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
