package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gss/configs"
	deliveryhttp "gss/internal/delivery/http"
	"gss/internal/delivery/http/handler"
	authhandler "gss/internal/delivery/http/handler/auth"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
	"gss/internal/repository"
	"gss/pkg/database/mysql"
)

type App struct {
	cfg             *configs.Config
	log             *logger.Logger
	sqlDB           *sql.DB
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

func New() *App {
	cfg := configs.Get()

	log := logger.New(
		logger.WithLevel(cfg.Logger.Level),
	)

	return &App{
		cfg: cfg,
		log: log,
	}
}

func (a *App) Start() {
	ctx := context.Background()

	a.initDB(ctx)
	a.initHTTPServer(ctx)

	// Start
	errCh := make(chan error, 1)
	go func() {
		a.log.InfoContext(ctx, "HTTP server starting", "addr", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		a.log.InfoContext(ctx, "Received signal", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			a.log.ErrorContext(ctx, "Server error", "error", err)
		}
	}

	a.Shutdown(ctx)
}

func (a *App) Shutdown(ctx context.Context) {
	a.log.InfoContext(ctx, "Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(ctx, a.shutdownTimeout)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		a.log.ErrorContext(ctx, "Server forced to shutdown", "error", err)
	}

	if a.sqlDB != nil {
		mysql.Close(a.sqlDB)
	}

	a.log.InfoContext(ctx, "Server exited")
}

func (a *App) initDB(ctx context.Context) {
	a.log.InfoContext(ctx, "Connecting to database...")

	sqlDB, err := mysql.New(
		mysql.WithDSN(a.cfg.MySQL.Host, a.cfg.MySQL.Port, a.cfg.MySQL.User, a.cfg.MySQL.Password, a.cfg.MySQL.Database),
		mysql.WithMaxOpenConns(a.cfg.MySQL.MaxOpenConns),
		mysql.WithMaxIdleConns(a.cfg.MySQL.MaxIdleConns),
		mysql.WithConnMaxLifetime(a.cfg.MySQL.ConnMaxLifetime),
		mysql.WithConnMaxIdleTime(a.cfg.MySQL.ConnMaxIdleTime),
	)
	if err != nil {
		a.log.FatalContext(ctx, "Failed to connect to database", "error", err)
	}
	a.sqlDB = sqlDB

	a.log.InfoContext(ctx, "Connected to database successfully")
}

func (a *App) initHTTPServer(ctx context.Context) {
	db, err := orm.NewDB(a.sqlDB, "mysql")
	if err != nil {
		a.log.FatalContext(ctx, "Failed to initialize ORM", "error", err)
	}

	ormLogger := orm.New(
		orm.WithLogger(a.log),
		orm.WithLevel(a.cfg.Logger.Level),
	)
	db.AddQueryHook(ormLogger)

	// Repositories
	userRepo := repository.NewUserRepository(db, a.log)

	// Handlers
	baseHandler := handler.NewHandler(a.cfg.Logger.Level)
	authHandler := authhandler.NewHandler(baseHandler, userRepo, 10*time.Second, a.log)

	// Router
	router := deliveryhttp.NewRouter(authHandler, a.log, "1.0.0")

	// Shutdown timeout
	a.shutdownTimeout = time.Duration(a.cfg.Server.ShutdownTimeout) * time.Second
	if a.shutdownTimeout == 0 {
		a.shutdownTimeout = 10 * time.Second
	}

	a.httpServer = &http.Server{
		Addr:    ":" + a.cfg.Server.Port,
		Handler: router.Handler(),
	}
}
