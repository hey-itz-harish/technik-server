package main

import (
	"technik-server/config"
	"technik-server/database"
	"technik-server/logger"
	"technik-server/routes"
)

func main() {
	// 1. Initialize structured logger
	logger.InitLogger()

	// 2. Load application environment configuration
	cfg := config.LoadConfig()

	// 3. Initialize Prisma Database Client
	database.InitDB()
	defer database.DisconnectDB()

	// 4. Setup router with CORS, CSRF, Logger, Error Handler, and API routes
	router := routes.SetupRouter(cfg)

	// 5. Start HTTP Server
	logger.Info("Technik Go Server running on port :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		logger.Error("Server failed to start: %v", err)
	}
}
