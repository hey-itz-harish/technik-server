package database

import (
	"log"

	"technik-server/prisma/db"
)

var Client *db.PrismaClient

// InitDB initializes and connects the Prisma Go client to PostgreSQL
func InitDB() *db.PrismaClient {
	Client = db.NewClient()
	if err := Client.Prisma.Connect(); err != nil {
		log.Printf("Warning: Failed to connect to PostgreSQL database via Prisma: %v", err)
		log.Println("Make sure PostgreSQL is running and your DATABASE_URL in .env is correct.")
		return Client
	}

	log.Println("Successfully connected to PostgreSQL database via Prisma Client Go!")
	return Client
}

// DisconnectDB closes the database connection
func DisconnectDB() {
	if Client != nil {
		if err := Client.Prisma.Disconnect(); err != nil {
			log.Printf("Error disconnecting Prisma Client: %v", err)
		} else {
			log.Println("Prisma database client disconnected cleanly.")
		}
	}
}
