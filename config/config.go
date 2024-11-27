package config

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Config struct {
	DB       *sql.DB
	Port     string
	DBConfig PostgresConfig
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

func NewConfig() *Config {
	return &Config{
		Port: "8080",
		DBConfig: PostgresConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "bookstore_user",
			Password: "your_password",
			DBName:   "bookstore_db",
		},
	}
}

func (c *Config) ConnectDB() error {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBConfig.Host,
		c.DBConfig.Port,
		c.DBConfig.User,
		c.DBConfig.Password,
		c.DBConfig.DBName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}

	c.DB = db
	return nil
}
