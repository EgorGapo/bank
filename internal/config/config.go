package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Postgres Postgres
}

type Postgres struct {
	Host     string `env:"POSTGRES_HOST"     envDefault:"localhost"`
	Port     string `env:"POSTGRES_PORT"     envDefault:"5432"`
	User     string `env:"POSTGRES_USER,required"`
	Password string `env:"POSTGRES_PASSWORD,required"`
	DB       string `env:"POSTGRES_DB,required"`
	SSLMode  string `env:"POSTGRES_SSLMODE"  envDefault:"disable"`
	MaxConn  int32  `env:"POSTGRES_MAX_CONN" envDefault:"10"`
}

func New() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}

func (p Postgres) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode)
}
