package main

import (
	"log/slog"
	"os"

	"github.com/user/crawler-service/pkg/logger"
)

func main() {
	logger.Init(os.Stdout, slog.LevelInfo)
	slog.Info("Starting crawler service...")
	// TODO: Initialize config, database, redis, and start the server
}

