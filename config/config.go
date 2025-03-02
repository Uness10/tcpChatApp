package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DB        PostgresConfig
	Server    ServerConfig
	JWTSecret string
	Env       string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type ServerConfig struct {
	Address string
}

var AppConfig Config

func InitConfig() {
	AppConfig = Load()
	log.Println("✅ Config loaded successfully!")
}

func Load() Config {
	err := godotenv.Load()
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("⚠️ No .env file found. Using system environment variables.")
		} else {
			log.Fatalf("❌ Error loading .env file: %v", err)
		}
	}

	return Config{
		Server: ServerConfig{
			Address: getEnv("SERVER_HOST", "127.0.0.1") + ":" + getEnv("SERVER_PORT", "8088"),
		},
		DB: PostgresConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "chatdb"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWTSecret: getEnv("JWT_SECRET", "default-secret"),
		Env:       getEnv("ENV", "production"),
	}
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
