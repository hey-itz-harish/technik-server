package database

import (
	"database/sql"
	"log"
	"os"
)

// EnsureAcademicAssessmentTables creates tables for Academic Assessments and Student Evaluations
func EnsureAcademicAssessmentTables() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}

	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Error opening database for Academic Assessment schema: %v", err)
		return
	}
	defer rawDB.Close()

	queries := []string{
		// 1. Academic Assessments Table
		`CREATE TABLE IF NOT EXISTS academic_assessments (
			id VARCHAR(64) PRIMARY KEY,
			school_name VARCHAR(255) NOT NULL,
			school_id VARCHAR(64) NOT NULL,
			exam_name VARCHAR(255) NOT NULL,
			exam_question_paper_document_path TEXT NOT NULL DEFAULT '',
			class VARCHAR(100) NOT NULL,
			exam_date VARCHAR(50) NOT NULL,
			number_of_questions INT NOT NULL DEFAULT 0,
			total_mark NUMERIC(6, 2) NOT NULL DEFAULT 0,
			questions JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) DEFAULT '',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_by VARCHAR(255) DEFAULT '',
			deleted_at TIMESTAMP,
			deleted_by VARCHAR(255) DEFAULT ''
		);`,

		// 2. Student Academic Evaluations Table (stores the 6-property answer sheet grid per student)
		`CREATE TABLE IF NOT EXISTS student_academic_evaluations (
			id VARCHAR(64) PRIMARY KEY,
			assessment_id VARCHAR(64) NOT NULL REFERENCES academic_assessments(id) ON DELETE CASCADE,
			student_id VARCHAR(64) NOT NULL,
			student_name VARCHAR(255) NOT NULL,
			school_id VARCHAR(64) NOT NULL,
			school_name VARCHAR(255) NOT NULL,
			class VARCHAR(100) NOT NULL,
			exam_name VARCHAR(255) NOT NULL,
			exam_date VARCHAR(50) NOT NULL,
			grid_data JSONB NOT NULL DEFAULT '[]'::jsonb,
			total_marks NUMERIC(6, 2) NOT NULL DEFAULT 0,
			obtained_marks NUMERIC(6, 2) NOT NULL DEFAULT 0,
			percentage NUMERIC(5, 2) NOT NULL DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'Pending',
			debar_reason TEXT DEFAULT '',
			evaluator_notes TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) DEFAULT '',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_by VARCHAR(255) DEFAULT '',
			deleted_at TIMESTAMP,
			deleted_by VARCHAR(255) DEFAULT ''
		);`,

		// 3. Add assessment_grid & status columns to StudentDetails table if not present
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "assessment_grid" JSONB DEFAULT '[]'::jsonb;`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "assessment_status" TEXT DEFAULT 'Pending';`,
		`ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "academic_status" TEXT DEFAULT 'Active';`,

		// Indices for rapid query lookups
		`CREATE INDEX IF NOT EXISTS idx_academic_assessments_school_id ON academic_assessments(school_id);`,
		`CREATE INDEX IF NOT EXISTS idx_academic_assessments_class ON academic_assessments(class);`,
		`CREATE INDEX IF NOT EXISTS idx_student_evaluations_assessment_id ON student_academic_evaluations(assessment_id);`,
		`CREATE INDEX IF NOT EXISTS idx_student_evaluations_student_id ON student_academic_evaluations(student_id);`,
		`CREATE INDEX IF NOT EXISTS idx_student_evaluations_school_id ON student_academic_evaluations(school_id);`,
		`CREATE INDEX IF NOT EXISTS idx_student_evaluations_status ON student_academic_evaluations(status);`,
	}

	for _, q := range queries {
		if _, err := rawDB.Exec(q); err != nil {
			log.Printf("Warning executing Academic Assessment schema query: %v", err)
		}
	}

	log.Println("Academic Assessment tables and schema verified and migrated successfully.")
}

