package configs

import (
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config is the root configuration container aggregating all sub-configs.
type Config struct {
	HTTP       HTTPConfig       `mapstructure:",squash"`
	Logger     LoggerConfig     `mapstructure:",squash"`
	MySQL      MySQLConfig      `mapstructure:",squash"`
	WorkerPool WorkerPoolConfig `mapstructure:",squash"`
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Port            string        `mapstructure:"HTTP_PORT"`
	ReadTimeout     time.Duration `mapstructure:"HTTP_READ_TIMEOUT"`
	WriteTimeout    time.Duration `mapstructure:"HTTP_WRITE_TIMEOUT"`
	IdleTimeout     time.Duration `mapstructure:"HTTP_IDLE_TIMEOUT"`
	ShutdownTimeout time.Duration `mapstructure:"HTTP_SHUTDOWN_TIMEOUT"`
	RequestTimeout  time.Duration `mapstructure:"HTTP_REQUEST_TIMEOUT"`
}

// LoggerConfig holds logging settings.
type LoggerConfig struct {
	Level string `mapstructure:"LOGGER_LEVEL"`
}

// MySQLConfig holds MySQL connection settings.
type MySQLConfig struct {
	Host            string `mapstructure:"MYSQL_HOST"`
	Port            int    `mapstructure:"MYSQL_PORT"`
	User            string `mapstructure:"MYSQL_USER"`
	Password        string `mapstructure:"MYSQL_PASSWORD"`
	Database        string `mapstructure:"MYSQL_DATABASE"`
	MaxOpenConns    int    `mapstructure:"MYSQL_MAX_OPEN_CONNS"`
	MaxIdleConns    int    `mapstructure:"MYSQL_MAX_IDLE_CONNS"`
	ConnMaxLifetime int    `mapstructure:"MYSQL_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime int    `mapstructure:"MYSQL_CONN_MAX_IDLE_TIME"`
}

// WorkerPoolConfig holds worker pool settings.
type WorkerPoolConfig struct {
	Workers   int `mapstructure:"WORKER_POOL_WORKERS"`
	QueueSize int `mapstructure:"WORKER_POOL_QUEUE_SIZE"`
}

// --- Singleton loader ---

var ptr atomic.Pointer[Config]

func init() {
	ptr.Store(&Config{})
}

// Load reads the .env file, parses it into Config, and sets up hot-reload.
func Load() error {
	v := viper.New()

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	cfg, err := parse(v)
	if err != nil {
		return err
	}
	ptr.Store(cfg)

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		cfg, err := parse(v)
		if err == nil {
			ptr.Store(cfg)
		}
	})

	return nil
}

func parse(v *viper.Viper) (*Config, error) {
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Get returns the current configuration snapshot.
func Get() *Config {
	return ptr.Load()
}
