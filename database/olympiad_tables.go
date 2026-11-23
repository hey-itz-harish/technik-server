package database

import (
	"database/sql"
	"log"
	"os"
)

// EnsureOlympiadTables creates all required tables for Olympiad Tests, Questions, Participants, and Student Test Status
func EnsureOlympiadTables() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}

	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Error opening database for Olympiad schema: %v", err)
		return
	}
	defer rawDB.Close()

	queries := []string{
		// 1. Olympiad Tests Table
		`CREATE TABLE IF NOT EXISTS olympiad_tests (
			id VARCHAR(64) PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			track VARCHAR(100) NOT NULL,
			class_category VARCHAR(100) NOT NULL,
			start_date VARCHAR(30) NOT NULL,
			end_date VARCHAR(30) NOT NULL,
			start_time VARCHAR(20) NOT NULL,
			end_time VARCHAR(20) NOT NULL,
			duration_minutes INT NOT NULL DEFAULT 60,
			total_marks INT NOT NULL DEFAULT 100,
			passing_marks INT NOT NULL DEFAULT 40,
			instructions TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// 2. Olympiad Questions Table
		`CREATE TABLE IF NOT EXISTS olympiad_questions (
			id VARCHAR(64) PRIMARY KEY,
			test_id VARCHAR(64) NOT NULL REFERENCES olympiad_tests(id) ON DELETE CASCADE,
			question_number INT NOT NULL,
			question_text TEXT NOT NULL,
			image_url TEXT DEFAULT '',
			options JSONB NOT NULL DEFAULT '[]'::jsonb,
			correct_option INT NOT NULL DEFAULT 0,
			marks INT NOT NULL DEFAULT 4,
			explanation TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// 3. Rename olympiad_student_allocations -> olympiadstudentteststatus if legacy table exists
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_name = 'olympiad_student_allocations'
			) AND NOT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_name = 'olympiadstudentteststatus'
			) THEN
				ALTER TABLE olympiad_student_allocations RENAME TO olympiadstudentteststatus;
			END IF;
		END $$;`,

		// 4. Olympiad Student Test Status Table (renamed from olympiad_student_allocations)
		`CREATE TABLE IF NOT EXISTS olympiadstudentteststatus (
			id VARCHAR(64) PRIMARY KEY,
			test_id VARCHAR(64) NOT NULL REFERENCES olympiad_tests(id) ON DELETE CASCADE,
			student_name VARCHAR(255) NOT NULL,
			student_email VARCHAR(255) NOT NULL,
			student_phone VARCHAR(50) DEFAULT '',
			school_name VARCHAR(255) DEFAULT '',
			grade VARCHAR(50) DEFAULT '',
			access_code VARCHAR(30) UNIQUE NOT NULL,
			scheduled_date VARCHAR(30) DEFAULT '',
			allocated_duration INT NOT NULL DEFAULT 60,
			time_remaining INT NOT NULL DEFAULT 3600,
			status VARCHAR(50) NOT NULL DEFAULT 'NOT_STARTED',
			resume_allowed BOOLEAN NOT NULL DEFAULT FALSE,
			current_question INT NOT NULL DEFAULT 0,
			answers JSONB NOT NULL DEFAULT '{}'::jsonb,
			proctor_logs JSONB NOT NULL DEFAULT '[]'::jsonb,
			screen_share_active BOOLEAN NOT NULL DEFAULT FALSE,
			started_at TIMESTAMP,
			last_heartbeat_at TIMESTAMP,
			completed_at TIMESTAMP,
			score INT DEFAULT 0,
			total_questions INT DEFAULT 0,
			admin_notes TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// 5. Create olympiadParticipants Table
		`CREATE TABLE IF NOT EXISTS "olympiadParticipants" (
			id TEXT PRIMARY KEY,
			"studentName" TEXT NOT NULL,
			"studentId" TEXT,
			"schoolName" TEXT,
			"schoolCode" TEXT,
			gender TEXT NOT NULL,
			"olympiadTrack" TEXT NOT NULL,
			"createdAt" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			"createdBy" VARCHAR(255) DEFAULT '',
			test_id VARCHAR(64) REFERENCES olympiad_tests(id) ON DELETE SET NULL,
			olympiadquestionsid TEXT,
			olympiadstudentteststatusid VARCHAR(64) REFERENCES olympiadstudentteststatus(id) ON DELETE SET NULL,
			"updatedAt" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			"updatedBy" VARCHAR(255) DEFAULT '',
			deleted BOOLEAN NOT NULL DEFAULT FALSE,
			"deletedAt" TIMESTAMP
		);`,

		// Drop old OlympiadStudent & OlympiadRegistration tables if migrating
		`DROP TABLE IF EXISTS "OlympiadStudent" CASCADE;`,
		`DROP TABLE IF EXISTS "OlympiadRegistration" CASCADE;`,

		// Alter additions for dates, template & certificates
		`ALTER TABLE olympiad_tests ADD COLUMN IF NOT EXISTS registration_start_date VARCHAR(30) DEFAULT '';`,
		`ALTER TABLE olympiad_tests ADD COLUMN IF NOT EXISTS registration_end_date VARCHAR(30) DEFAULT '';`,
		`ALTER TABLE olympiad_tests ADD COLUMN IF NOT EXISTS certificate_template TEXT DEFAULT '';`,
		`ALTER TABLE olympiadstudentteststatus ADD COLUMN IF NOT EXISTS certificate_issued BOOLEAN DEFAULT FALSE;`,
		`ALTER TABLE olympiadstudentteststatus ADD COLUMN IF NOT EXISTS certificate_url TEXT DEFAULT '';`,

		// Indices for rapid query lookup
		`CREATE INDEX IF NOT EXISTS idx_olympiad_questions_test_id ON olympiad_questions(test_id);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_status_test_id ON olympiadstudentteststatus(test_id);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_status_code ON olympiadstudentteststatus(access_code);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_status_email ON olympiadstudentteststatus(student_email);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_participants_test_id ON "olympiadParticipants"(test_id);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_participants_student_id ON "olympiadParticipants"("studentId");`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_participants_school_code ON "olympiadParticipants"("schoolCode");`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_participants_status_id ON "olympiadParticipants"(olympiadstudentteststatusid);`,
	}

	for _, q := range queries {
		if _, err := rawDB.Exec(q); err != nil {
			log.Printf("Warning executing Olympiad table migration query: %v", err)
		}
	}

	log.Println("Olympiad proctored portal database tables (olympiadParticipants, olympiadstudentteststatus) verified and migrated successfully.")
}
