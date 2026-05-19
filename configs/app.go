package configs

import "time"

type AppConfig struct {
	ShutdownTimeout time.Duration `mapstructure:"APP_SHUTDOWN_TIMEOUT"`
}
