package main

import (
	"encoding/json"
	"os"

	"go.uber.org/zap"
)

// Config stores app global configuration
type Config struct {
	RedisAuth    string `json:"redis_auth"`
	RedisHost    string `json:"redis_host"`
	RedisPort    string `json:"redis_port"`
	RedisDBID    int    `json:"redis_dbid"`
	Domain       string `json:"domain"`
	DiscordToken string `json:"discord_token"`
	DiscordBinds []Bind `json:"discord_binds"`
}

var logger *zap.Logger

func init() {
	var err error
	var config zap.Config

	dyn := zap.NewAtomicLevel()
	dyn.SetLevel(zap.DebugLevel)
	config.Level = dyn
	config = zap.NewDevelopmentConfig()
	config.DisableCaller = true

	logger, err = config.Build()
	if err != nil {
		panic(err)
	}
}

func main() {
	cfg := loadConfig("config.json")

	logger.Info("initialising goss", zap.Any("config", cfg))

	Start(cfg)
}

func loadConfig(filename string) Config {
	var config Config

	file, err := os.Open(filename)
	if err != nil {
		logger.Fatal("failed to open config file",
			zap.Error(err))
	}

	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		logger.Fatal("failed to decode config file",
			zap.Error(err))
	}

	err = file.Close()
	if err != nil {
		logger.Fatal("failed to close config file",
			zap.Error(err))
	}

	return config
}
