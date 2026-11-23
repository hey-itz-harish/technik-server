package database

import (
	"database/sql"
	"log"
	"os"
)

// EnsurePortalRoleTables creates the roles table strictly for Technik Portal users
// (applicable only to Technik Portal staff/admins, NOT for school login or student login)
// and ensures the deleted flag exists on the User table as default false for soft delete.
func EnsurePortalRoleTables() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}

	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Error opening database for Portal Roles schema: %v", err)
		return
	}
	defer rawDB.Close()

	queries := []string{
		// 1. Portal Roles Table (Strictly for Technik Portal Users)
		`CREATE TABLE IF NOT EXISTS roles (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			code VARCHAR(50) NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			portal_only BOOLEAN NOT NULL DEFAULT TRUE,
			permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) DEFAULT 'SYSTEM',
			updated_by VARCHAR(255) DEFAULT 'SYSTEM',
			deleted BOOLEAN NOT NULL DEFAULT FALSE,
			deleted_at TIMESTAMP
		);`,

		// 2. Ensure "deleted" flag (default false) for soft delete exists on "User" table
		`ALTER TABLE IF EXISTS "User" ADD COLUMN IF NOT EXISTS "deleted" BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE IF EXISTS "User" ADD COLUMN IF NOT EXISTS "role_id" VARCHAR(64);`,
		`ALTER TABLE IF EXISTS "User" ADD COLUMN IF NOT EXISTS "role_name" VARCHAR(100) DEFAULT 'Admin';`,
		`ALTER TABLE IF EXISTS "User" ADD COLUMN IF NOT EXISTS "zone" VARCHAR(100) DEFAULT '';`,
		`ALTER TABLE IF EXISTS "User" ADD COLUMN IF NOT EXISTS "mobile" VARCHAR(50) DEFAULT '';`,

		// Handle table name if lowercase "users"
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS deleted BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS role_id VARCHAR(64);`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS role_name VARCHAR(100) DEFAULT 'Admin';`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS zone VARCHAR(100) DEFAULT '';`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS mobile VARCHAR(50) DEFAULT '';`,

		// Indices for rapid query lookup
		`CREATE INDEX IF NOT EXISTS idx_roles_code ON roles(code);`,
		`CREATE INDEX IF NOT EXISTS idx_roles_portal_only ON roles(portal_only);`,
		`CREATE INDEX IF NOT EXISTS idx_roles_deleted ON roles(deleted);`,
	}

	for _, q := range queries {
		if _, err := rawDB.Exec(q); err != nil {
			log.Printf("Warning executing Portal Roles schema query: %v", err)
		}
	}

	// Seed default portal roles if empty
	seedPortalRoles(rawDB)

	log.Println("Portal Roles table and User deleted flag verified and migrated successfully.")
}

func seedPortalRoles(db *sql.DB) {
	type defaultRole struct {
		id          string
		name        string
		code        string
		description string
		permissions string
	}

	defaultRoles := []defaultRole{
		{
			id:          "ROLE-SUPER-ADMIN",
			name:        "Super Admin",
			code:        "SUPER_ADMIN",
			description: "Full administrative control across all Technik Portal operations, assessments, proctored exams, and school accounts.",
			permissions: `["all", "users.manage", "roles.manage", "schools.manage", "olympiad.manage", "pride.manage", "assessment.manage", "news.manage"]`,
		},
		{
			id:          "ROLE-REGIONAL-ADMIN",
			name:        "Regional Olympiad Admin",
			code:        "REGIONAL_ADMIN",
			description: "Oversees regional Olympiad batches, exam centers, student allocations, access codes, and score validations.",
			permissions: `["olympiad.manage", "olympiad.allocate", "olympiad.monitor", "reports.view"]`,
		},
		{
			id:          "ROLE-PRIDE-EVALUATOR",
			name:        "Pride Award Evaluator",
			code:        "PRIDE_EVALUATOR",
			description: "Evaluates and reviews Technik Pride Award student nominations, projects, portfolios, and awards status.",
			permissions: `["pride.review", "pride.evaluate", "pride.status_update"]`,
		},
		{
			id:          "ROLE-ACADEMIC-EVALUATOR",
			name:        "Academic Assessment Evaluator",
			code:        "ACADEMIC_EVALUATOR",
			description: "Evaluates school academic assessment papers, marks answer sheet grids, and approves promotions or debarments.",
			permissions: `["assessment.view", "assessment.evaluate", "assessment.promote", "assessment.debar"]`,
		},
		{
			id:          "ROLE-EXAM-INSPECTOR",
			name:        "Exam Center Inspector",
			code:        "EXAM_INSPECTOR",
			description: "Monitors live proctored student examinations, audio/video proctor logs, and anti-cheating audit trails.",
			permissions: `["olympiad.monitor", "olympiad.audit_logs", "proctor.view"]`,
		},
		{
			id:          "ROLE-CONTENT-MANAGER",
			name:        "Content & Communications Manager",
			code:        "CONTENT_MANAGER",
			description: "Publishes and updates official news circulars, press releases, banner alerts, and event dates.",
			permissions: `["news.publish", "news.edit", "news.delete"]`,
		},
	}

	insertQuery := `
		INSERT INTO roles (
			id, name, code, description, portal_only, permissions, is_active, created_by, updated_by
		) VALUES ($1, $2, $3, $4, TRUE, $5::jsonb, TRUE, 'SYSTEM', 'SYSTEM')
		ON CONFLICT (code) DO UPDATE 
		SET name = EXCLUDED.name, description = EXCLUDED.description, portal_only = TRUE;
	`

	for _, r := range defaultRoles {
		_, _ = db.Exec(insertQuery, r.id, r.name, r.code, r.description, r.permissions)
	}
}
