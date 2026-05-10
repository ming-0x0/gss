package configs

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
