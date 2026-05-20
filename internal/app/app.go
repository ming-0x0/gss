package app

import (
	"context"
	"gss/configs"
	deliveryHTTP "gss/internal/delivery/http"
	"gss/internal/delivery/http/handler/auth"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
	"gss/internal/repository"
	"gss/pkg/database/mysql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type App struct {
	cfg    *configs.Config
	db     *orm.DB
	srv    *http.Server
	logger *logger.Logger
	wg     sync.WaitGroup
}

func New(
	version string,
	cfg *configs.Config,
) (*App, error) {
	logger := logger.New()
	slog.SetDefault(logger.Logger)

	logger.Info("Connecting to database...")
	mysqlConn, err := mysql.Open(
		mysql.WithDSN(cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Database),
		mysql.WithMaxIdleConns(cfg.MySQL.MaxIdleConns),
		mysql.WithMaxOpenConns(cfg.MySQL.MaxOpenConns),
		mysql.WithConnMaxLifetime(cfg.MySQL.ConnMaxLifetime),
		mysql.WithConnMaxIdleTime(cfg.MySQL.ConnMaxIdleTime),
	)
	if err != nil {
		return nil, err
	}

	db, err := orm.NewDB(mysqlConn, "mysql", orm.WithLogger(
		orm.NewLogger(
			orm.WithLoggerHandler(logger.Handler()),
		),
	))
	if err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(db, logger)
	authHandler := auth.NewHandler(userRepo, logger)

	handlers := []deliveryHTTP.Handler{
		authHandler,
	}

	router, err := deliveryHTTP.NewRouter(
		deliveryHTTP.WithLogger(logger),
		deliveryHTTP.WithVersion(version),
		deliveryHTTP.WithHandlers(handlers...),
	)
	if err != nil {
		return nil, err
	}

	return &App{
		cfg:    cfg,
		db:     db,
		logger: logger,
		srv: &http.Server{
			Addr:         ":" + cfg.HTTP.Port,
			Handler:      router.Handler(),
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
			IdleTimeout:  cfg.HTTP.IdleTimeout,
		},
	}, nil
}

func (a *App) Run() error {
	defer a.wg.Wait()
	return a.start()
}

func (a *App) start() error {
	a.stop(a.cfg.App.ShutdownTimeout, func(ctx context.Context) error {
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
