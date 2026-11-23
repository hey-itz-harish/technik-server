package main

import (
	"log"
	"os"

	"technik-server/config"
	"technik-server/database"
	"technik-server/logger"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env explicitly if available
	_ = godotenv.Load()
	if os.Getenv("DATABASE_URL") == "" {
		_ = godotenv.Load("../../.env")
	}

	logger.InitLogger()
	_ = config.LoadConfig()

	log.Println("Starting database schema migration...")
	database.AutoMigrateSchema()
	log.Println("Database schema migration completed successfully!")
}
