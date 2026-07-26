package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DBSource             string        `mapstructure:"DB_SOURCE"`
	ServerAddress        string        `mapstructure:"SERVER_ADDRESS"`
	TokenKey             string        `mapstructure:"TOKEN_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
	Environment          string        `mapstructure:"ENVIRONMENT"`
	RbmUrl               string        `mapstructure:"RBM_URL"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&config)
	return
}

func LoadKuberConfig() (config Config, err error) {
	accessDurationStr := os.Getenv("ACCESS_TOKEN_DURATION")
	if accessDurationStr == "" {
		return config, fmt.Errorf("ACCESS_TOKEN_DURATION not set")
	}
	accessDuration, err := time.ParseDuration(accessDurationStr)
	if err != nil {
		return config, err
	}

	refreshDurationStr := os.Getenv("REFRESH_TOKEN_DURATION")
	if refreshDurationStr == "" {
		return config, fmt.Errorf("REFRESH_TOKEN_DURATION not set")
	}
	refreshDuration, err := time.ParseDuration(refreshDurationStr)
	if err != nil {
		return config, err
	}

	return Config{
		DBSource:             os.Getenv("DB_SOURCE"),
		ServerAddress:        os.Getenv("SERVER_ADDRESS"),
		TokenKey:             os.Getenv("TOKEN_KEY"),
		AccessTokenDuration:  accessDuration,
		RefreshTokenDuration: refreshDuration,
		Environment:          os.Getenv("ENVIRONMENT"),
		RbmUrl:               os.Getenv("RBM_URL"),
	}, nil
}
