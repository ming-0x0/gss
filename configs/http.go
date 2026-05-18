package configs

type HTTPConfig struct {
	Port           string `mapstructure:"HTTP_PORT"`
	ReadTimeout    int    `mapstructure:"HTTP_READ_TIMEOUT"`
	WriteTimeout   int    `mapstructure:"HTTP_WRITE_TIMEOUT"`
	IdleTimeout    int    `mapstructure:"HTTP_IDLE_TIMEOUT"`
	RequestTimeout int    `mapstructure:"HTTP_REQUEST_TIMEOUT"`
}
