package main

import (
	"collector/models"
	"collector/utils"
	"os"
)

func main() {
	log := utils.GlobalLogger()
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
}
