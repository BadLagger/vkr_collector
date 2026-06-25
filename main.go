package main

import (
	"collector/models"
	"collector/utils"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log := utils.GlobalLogger()
	log.SetLevel(utils.Debug)
	log.Info("Start Collector!")
	defer log.Info("Collector Ends!")

	if len(os.Args) < 2 {
		log.Error("Should set path to the config file!")
		return
	}

	cfg, err := models.NewConfig(os.Args[1])
	if err != nil {
		log.Error("Config error: +%v", err)
		return
	}

	log.Info("Socket path %s", cfg.UDSSocketPath)

	collectors := models.NewDataCollector(cfg)
	if err := collectors.Start(); err != nil {
		log.Error("start collector error: +%v", err)
		os.Exit(1)
	}

	log.Info("Data collector started")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info("Shutting down gracefully...")

	if err := collectors.Stop(); err != nil {
		log.Error("Error during shutdown: %v", err)
		os.Exit(1)
	}
	log.Info("Shutdown completed")
}
