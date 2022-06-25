package config

import (
	"log"
	"sync"

	"github.com/caarlos0/env"
)

type Config struct {
	Database struct {
		User     string `env:"DB_USER" envDefault:"admin"`
		Password string `env:"DB_PASSWORD"`
		Host     string `env:"DB_HOST" envDefault:"localhost"`
		Port     string `env:"DB_PORT" envDefault:"5432"`
		Name     string `env:"DB_NAME" envDefault:"db"`
	}
	Server struct {
		Host string `env:"HOST" envDefault:"0.0.0.0"`
		Port string `env:"PORT" envDefault:"8080"`
	}
	Indexer struct {
		RpcUrl string `env:"RPC_URL"`
	}
}

var config *Config
var once sync.Once

func GetConfig() *Config {
	once.Do(func() {
		config = &Config{}
		if err := env.Parse(config); err != nil {
			log.Fatalf("%+v\n", err)
		}
	})
	return config
}
