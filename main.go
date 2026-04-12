package main

import "collector/utils"

func main() {
	log := utils.GlobalLogger()
	log.Info("Start Collector!")
	defer log.Info("Collector Ends!")
}