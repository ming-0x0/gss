package app

import (
	"context"
	"database/sql"
	"gss/configs"
	deliveryHTTP "gss/internal/delivery/http"
	"gss/internal/delivery/http/handler"
	"gss/internal/delivery/http/handler/auth"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
	"gss/internal/repository"
	"gss/pkg/database/mysql"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type App struct {
	cfg    *configs.Config
	db     *sql.DB
	srv    *http.Server
	logger *logger.Logger
	wg     sync.WaitGroup
}

func New(
	version string,
	cfg *configs.Config,
	logger *logger.Logger,
) (*App, error) {
	logger.Info("Connecting to mysql database...")
	mysqlDB, err := mysql.New(
		mysql.WithDSN(cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Database),
		mysql.WithMaxIdleConns(cfg.MySQL.MaxIdleConns),
		mysql.WithMaxOpenConns(cfg.MySQL.MaxOpenConns),
		mysql.WithConnMaxLifetime(cfg.MySQL.ConnMaxLifetime),
		mysql.WithConnMaxIdleTime(cfg.MySQL.ConnMaxIdleTime),
	)
	if err != nil {
		return nil, err
	}

	db, err := orm.NewDB(mysqlDB, "mysql")
	if err != nil {
		return nil, err
	}

	repositoryContainer := repository.NewRepositoryContainer(db, logger)

	baseHandler := handler.NewHandler(cfg.Logger.Level)

	handlers := []deliveryHTTP.Handler{
		auth.NewHandler(baseHandler, repositoryContainer.UserRepository, logger),
	}

	router, err := deliveryHTTP.NewRouter(
		logger,
		version,
		handlers...,
	)
	if err != nil {
		return nil, err
	}

	return &App{
		cfg:    cfg,
		db:     mysqlDB,
		logger: logger,
		srv: &http.Server{
			Addr:    ":8080",
			Handler: router.Handler(),
		},
	}, nil
}

func (a *App) Run() error {
	defer a.wg.Wait()
	return a.start()
}

func (a *App) start() error {
	a.stop(10*time.Second, func(ctx context.Context) error {
		return a.srv.Shutdown(ctx)
	})

	if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (a *App) stop(
	timeout time.Duration,
	callback func(ctx context.Context) error,
) {
	a.wg.Go(func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(quit)

		sig := <-quit
		a.logger.Info("Received signal " + sig.String() + ". Shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := callback(ctx); err != nil {
			a.logger.Error("Error during graceful shutdown", "err", err)
		}

		if a.db != nil {
			a.logger.Info("Closing database connection...")
			if err := a.db.Close(); err != nil {
				a.logger.Error("Error closing database", "err", err)
			}
		}
	})
}
