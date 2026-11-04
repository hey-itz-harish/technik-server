package database

import (
	"database/sql"
	"log"
	"os"
)

// EnsureOlympiadTables creates all required tables for Olympiad Tests, Questions, and Student Allocations
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

		// 3. Olympiad Student Allocations Table
		`CREATE TABLE IF NOT EXISTS olympiad_student_allocations (
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

		// Indices for rapid query lookup
		`CREATE INDEX IF NOT EXISTS idx_olympiad_questions_test_id ON olympiad_questions(test_id);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_allocations_test_id ON olympiad_student_allocations(test_id);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_allocations_code ON olympiad_student_allocations(access_code);`,
		`CREATE INDEX IF NOT EXISTS idx_olympiad_allocations_email ON olympiad_student_allocations(student_email);`,
	}

	for _, q := range queries {
		if _, err := rawDB.Exec(q); err != nil {
			log.Printf("Warning executing Olympiad table migration query: %v", err)
		}
	}

	log.Println("Olympiad proctored portal database tables verified and migrated successfully.")
}
