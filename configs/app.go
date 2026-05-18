package configs

type AppConfig struct {
	ShutdownTimeout int `mapstructure:"APP_SHUTDOWN_TIMEOUT"`
}
