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

	userRepo := repository.NewUserRepository(db, logger)

	baseHandler := handler.NewHandler(cfg.Logger.Level)
	authHandler := auth.NewHandler(baseHandler, userRepo, logger)

	router := deliveryHTTP.NewRouter(
		logger,
		version,
		authHandler,
	)

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

func (s *App) Run() error {
	defer s.wg.Wait()
	return s.start()
}

func (s *App) start() error {
	s.stop(10*time.Second, func(ctx context.Context) error {
		return s.srv.Shutdown(ctx)
	})

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *App) stop(
	timeout time.Duration,
	callback func(ctx context.Context) error,
) {
	s.wg.Go(func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(quit)

		sig := <-quit
		s.logger.Info("Received signal " + sig.String() + ". Shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := callback(ctx); err != nil {
			s.logger.Error("Error during graceful shutdown", "err", err)
		}

		if s.db != nil {
			s.logger.Info("Closing database connection...")
			if err := s.db.Close(); err != nil {
				s.logger.Error("Error closing database", "err", err)
			}
		}
	})
}
