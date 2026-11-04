package database

import (
	"context"
	"database/sql"
	"log"
	"os"
	"sync"
	"time"

	"technik-server/prisma/db"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	Client *db.PrismaClient
	RawDB  *sql.DB
	mu     sync.Mutex
)

// GetRawDB returns a shared, thread-safe connection to the PostgreSQL database if reachable
func GetRawDB() (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, sql.ErrConnDone
	}

	if RawDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		defer cancel()
		if err := RawDB.PingContext(ctx); err == nil {
			return RawDB, nil
		}
		_ = RawDB.Close()
		RawDB = nil
		return nil, sql.ErrConnDone
	}

	dbConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if err := dbConn.PingContext(ctx); err != nil {
		_ = dbConn.Close()
		return nil, err
	}

	RawDB = dbConn
	return RawDB, nil
}

// InitDB initializes and connects the Prisma Go client to PostgreSQL
func InitDB() *db.PrismaClient {
	mu.Lock()
	defer mu.Unlock()

	if Client == nil {
		Client = db.NewClient()
	}

	if err := Client.Prisma.Connect(); err != nil {
		log.Printf("Warning: Failed to connect to PostgreSQL database via Prisma: %v", err)
		log.Println("Make sure PostgreSQL is running and your DATABASE_URL in .env is correct.")
		return Client
	}

	log.Println("Successfully connected to PostgreSQL database via Prisma Client Go!")
	AutoMigrateSchema()
	SeedSuperAdmin()
	return Client
}

// AutoMigrateSchema ensures all required columns exist on remote PostgreSQL tables
func AutoMigrateSchema() {
	EnsureOlympiadTables()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}

	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		return
	}
	defer rawDB.Close()

	alterQueries := []string{
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "nominationStatus" TEXT DEFAULT 'Submitted & Under Review';`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "adminStatus" TEXT DEFAULT 'Forwarded to Technik Super Admin';`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "classCategory" TEXT;`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "gender" TEXT;`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "category" TEXT;`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "classCategory" TEXT;`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "status" TEXT DEFAULT 'Registered & Verified';`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "academicYear" INT DEFAULT 2026;`,
		`ALTER TABLE "OlympiadStudent" ADD COLUMN IF NOT EXISTS "classCategory" TEXT;`,
	}

	for _, q := range alterQueries {
		_, _ = rawDB.Exec(q)
	}
	log.Println("PostgreSQL database schema columns verified & auto-migrated successfully.")
}

// SeedSuperAdmin creates the initial Super Admin user if not existing
func SeedSuperAdmin() {
	if Client == nil {
		return
	}
	ctx := context.Background()

	adminEmail := os.Getenv("INITIAL_ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = os.Getenv("ADMIN_EMAIL")
	}
	if adminEmail == "" {
		adminEmail = "hari@technikolympiad.com"
	}

	adminPassword := os.Getenv("INITIAL_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = os.Getenv("ADMIN_PASSWORD")
	}

	// If admin user already exists, do nothing
	user, err := Client.User.FindUnique(
		db.User.Email.Equals(adminEmail),
	).Exec(ctx)

	if err == nil && user != nil {
		return
	}

	// Avoid insecure default seeding in production if password is not configured
	if adminPassword == "" {
		if os.Getenv("GIN_MODE") == "release" {
			log.Println("Notice: INITIAL_ADMIN_PASSWORD not set. Skipping automatic admin seeding in production.")
			return
		}
		adminPassword = "TechnikPass#2026"
		log.Println("Notice: Auto-seeding initial admin using development password. Please set INITIAL_ADMIN_PASSWORD.")
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash admin password: %v", err)
		return
	}

	_, err = Client.User.CreateOne(
		db.User.Name.Set("Super Admin"),
		db.User.Email.Set(adminEmail),
		db.User.Password.Set(string(hashedPass)),
		db.User.Role.Set(db.RoleAdmin),
		db.User.IsActivated.Set(true),
		db.User.IsVerified.Set(true),
	).Exec(ctx)

	if err != nil {
		log.Printf("Error auto-seeding Super Admin user: %v", err)
	} else {
		log.Printf("Successfully initialized Super Admin user (%s)!", adminEmail)
	}
}

// EnsureConnected ensures that the Prisma client is connected to PostgreSQL, automatically reconnecting if needed.
func EnsureConnected() error {
	mu.Lock()
	defer mu.Unlock()

	if Client == nil {
		Client = db.NewClient()
		return Client.Prisma.Connect()
	}

	err := Client.Prisma.Connect()
	if err != nil {
		// Re-instantiate client if engine process crashed or disconnected
		log.Printf("Re-connecting Prisma Client Go: %v", err)
		Client = db.NewClient()
		err = Client.Prisma.Connect()
	}
	return err
}

// DisconnectDB closes the database connection
func DisconnectDB() {
	mu.Lock()
	defer mu.Unlock()

	if Client != nil {
		if err := Client.Prisma.Disconnect(); err != nil {
			log.Printf("Error disconnecting Prisma Client: %v", err)
		} else {
			log.Println("Prisma database client disconnected cleanly.")
		}
	}
}
