package configs

type ServerConfig struct {
	Port            string `mapstructure:"SERVER_PORT"`
	ShutdownTimeout int    `mapstructure:"SERVER_SHUTDOWN_TIMEOUT"` // seconds
}
