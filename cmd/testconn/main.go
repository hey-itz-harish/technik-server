package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	log.Printf("Original DB URL: %s", dbURL)

	// Railway requires sslmode=require and connect_timeout
	connStr := dbURL + "?sslmode=require&connect_timeout=10"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("sql.Open err: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Println("Testing Ping with sslmode=require...")
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	log.Println("Successfully connected to Railway PostgreSQL!")

	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== EXISTING TABLES IN DATABASE ===")
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			fmt.Printf("- %s\n", name)
		}
	}
}
