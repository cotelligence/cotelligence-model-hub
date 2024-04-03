package main

import (
	"cotelligence-model-hub/config"
	cm "cotelligence-model-hub/hub"
	"cotelligence-model-hub/log"

	"go.uber.org/zap"
)

func main() {
	config := config.GetConfig()

	// Initialize the Pod client
	cm.InitRunPodAPIClient("https://api.runpod.io/graphql", config.RunPodAPIKey)

	// TODO: Initialize the Cotelligence Pod client, when it is ready
	//cm.InitOrGetCotelligencePodClient(config.CotelligenceAPIKey)

	router := cm.SetupRouter()

	if err := router.Run(":8080"); err != nil {
		log.ZapLogger.Error("Failed to run server", zap.Error(err))
	}
}
