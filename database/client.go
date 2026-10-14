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
	SQLDB  *sql.DB
	mu     sync.Mutex
)

// configureConnectionPool sets production limits on open connections, idle connections, and lifetime
func configureConnectionPool(db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(15 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
}

// InitDB initializes and connects the Prisma Go client and raw PostgreSQL connection
func InitDB() *db.PrismaClient {
	mu.Lock()
	defer mu.Unlock()

	dbURL := os.Getenv("DATABASE_URL")
	if SQLDB == nil && dbURL != "" {
		raw, err := sql.Open("postgres", dbURL)
		if err == nil {
			configureConnectionPool(raw)
			SQLDB = raw
		}
	}

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

// GetSQLDB returns the active raw SQL database connection
func GetSQLDB() *sql.DB {
	mu.Lock()
	defer mu.Unlock()
	if SQLDB == nil {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL != "" {
			raw, err := sql.Open("postgres", dbURL)
			if err == nil {
				configureConnectionPool(raw)
				SQLDB = raw
			}
		}
	}
	return SQLDB
}

// AutoMigrateSchema ensures all required columns and tables exist on PostgreSQL
func AutoMigrateSchema() {
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
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "projectTitle" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "projectDescription" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "studentQuote" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "keyStrengths" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "developmentAreas" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "mentorRemarks" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "projectFile" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "studentPhoto" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "additionalFiles" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "paymentStatus" TEXT DEFAULT 'PENDING';`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "razorpayOrderId" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "razorpayPaymentId" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "razorpaySignature" TEXT;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "amountPaid" INT DEFAULT 0;`,
		`ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "paidAt" TIMESTAMPTZ;`,
		`CREATE TABLE IF NOT EXISTS "StudentAdditionalData" (
			"id" TEXT PRIMARY KEY,
			"student_id" TEXT UNIQUE NOT NULL,
			"student_name" TEXT,
			"school_id" TEXT,
			"project_title" TEXT,
			"project_description" TEXT,
			"student_quote" TEXT,
			"key_strengths" TEXT,
			"development_areas" TEXT,
			"mentor_remarks" TEXT,
			"project_file" TEXT,
			"student_photo" TEXT,
			"additional_files" TEXT,
			"achievement_score" DOUBLE PRECISION DEFAULT 4.8,
			"innovation_score" DOUBLE PRECISION DEFAULT 4.8,
			"creativity_score" DOUBLE PRECISION DEFAULT 4.3,
			"communication_score" DOUBLE PRECISION DEFAULT 4.3,
			"leadership_score" DOUBLE PRECISION DEFAULT 4.0,
			"social_impact_score" DOUBLE PRECISION DEFAULT 4.3,
			"payment_status" TEXT DEFAULT 'PENDING',
			"razorpay_order_id" TEXT,
			"razorpay_payment_id" TEXT,
			"razorpay_signature" TEXT,
			"amount_paid" INT DEFAULT 0,
			"paid_at" TIMESTAMPTZ,
			"created_at" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			"updated_at" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS "NewsEvents" (
			"id" TEXT PRIMARY KEY,
			"type" TEXT NOT NULL,
			"title" TEXT NOT NULL,
			"description" TEXT,
			"date" TEXT,
			"category" TEXT,
			"location" TEXT,
			"image_url" TEXT,
			"link_url" TEXT,
			"is_published" BOOLEAN DEFAULT true,
			"created_at" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			"updated_at" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range alterQueries {
		_, _ = rawDB.Exec(q)
	}

	log.Println("PostgreSQL database schema columns & NewsEvents table verified successfully.")
}

// SeedSuperAdmin creates the initial Super Admin user if not existing
func SeedSuperAdmin() {
	if Client == nil {
		return
	}
	ctx := context.Background()
	adminEmail := os.Getenv("ADMIN_INITIAL_EMAIL")
	if adminEmail == "" {
		adminEmail = "hari@technikolympiad.com"
	}

	user, err := Client.User.FindUnique(
		db.User.Email.Equals(adminEmail),
	).Exec(ctx)

	if err != nil || user == nil {
		adminPass := os.Getenv("ADMIN_INITIAL_PASSWORD")
		if adminPass == "" {
			log.Println("Note: ADMIN_INITIAL_PASSWORD not configured. Skipping initial Super Admin seeding.")
			return
		}

		hashedPass, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
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
			log.Printf("Successfully auto-seeded Super Admin user (%s)!", adminEmail)
		}
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

// DisconnectDB closes the database connection and raw SQL pool
func DisconnectDB() {
	mu.Lock()
	defer mu.Unlock()

	if SQLDB != nil {
		if err := SQLDB.Close(); err != nil {
			log.Printf("Error closing raw SQL DB pool: %v", err)
		} else {
			log.Println("Raw SQL DB connection pool closed cleanly.")
		}
		SQLDB = nil
	}

	if Client != nil {
		if err := Client.Prisma.Disconnect(); err != nil {
			log.Printf("Error disconnecting Prisma Client: %v", err)
		} else {
			log.Println("Prisma database client disconnected cleanly.")
		}
	}
}
