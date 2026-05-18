package app

import (
	"context"
	"database/sql"
	"gss/configs"
	deliveryHTTP "gss/internal/delivery/http"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
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
	logger.Info("Connecting to database...")
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

	db, err := orm.NewDB(mysqlDB, "mysql", orm.WithLogger(
		orm.NewLogger(
			orm.WithLoggerHandler(logger.Handler()),
			orm.WithLoggerLevel(cfg.Logger.Level),
		),
	))
	if err != nil {
		return nil, err
	}

	_ = db // TODO: inject db into repositories and handlers

	handlers := []deliveryHTTP.Handler{}

	router, err := deliveryHTTP.NewRouter(
		deliveryHTTP.WithLogger(logger),
		deliveryHTTP.WithVersion(version),
		deliveryHTTP.WithTimeout(cfg.HTTP.RequestTimeout),
		deliveryHTTP.WithHandlers(handlers...),
	)
	if err != nil {
		return nil, err
	}

	addr := cfg.HTTP.Port
	if addr == "" {
		addr = ":8080"
	}

	return &App{
		cfg:    cfg,
		db:     mysqlDB,
		logger: logger,
		srv: &http.Server{
			Addr:         addr,
			Handler:      router.Handler(),
			ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.HTTP.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(cfg.HTTP.IdleTimeout) * time.Second,
		},
	}, nil
}

func (a *App) Run() error {
	defer a.wg.Wait()
	return a.start()
}

func (a *App) start() error {
	timeout := a.cfg.App.ShutdownTimeout
	if timeout == 0 {
		timeout = 10
	}

	a.stop(time.Duration(timeout)*time.Second, func(ctx context.Context) error {
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
