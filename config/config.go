package config

type Config struct {
	DB     PostgresConfig
	Server ServerConfig
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type ServerConfig struct {
	Address string
}

func Load() Config {
	return Config{
		DB: PostgresConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "root",
			DBName:   "chatdb",
		},
		Server: ServerConfig{
			Address: "0.0.0.0:8888",
		},
	}
}
