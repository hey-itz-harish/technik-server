package database

import (
	"database/sql"
	"log"
	"os"
)

// EnsureCoordinatorTables creates the CoordinatorDetails table and decouples coordinator fields from SchoolDetails
func EnsureCoordinatorTables() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}

	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Error opening database for Coordinator schema: %v", err)
		return
	}
	defer rawDB.Close()

	queries := []string{
		// 1. Create CoordinatorDetails table
		`CREATE TABLE IF NOT EXISTS "CoordinatorDetails" (
			"id" TEXT PRIMARY KEY,
			"schoolId" TEXT REFERENCES "SchoolDetails"("id") ON DELETE CASCADE,
			"schoolName" TEXT,
			"schoolCode" TEXT,
			"principalName" TEXT,
			"coordinatorName" TEXT,
			"coordinatorDesignation" TEXT,
			"coordinatorMobile" TEXT,
			"coordinatorEmail" TEXT,
			"createdAt" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			"updatedAt" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			"deletedAt" TIMESTAMPTZ
		);`,

		// 2. Indices for quick school lookup
		`CREATE INDEX IF NOT EXISTS idx_coordinator_school_id ON "CoordinatorDetails"("schoolId");`,
		`CREATE INDEX IF NOT EXISTS idx_coordinator_school_code ON "CoordinatorDetails"("schoolCode");`,

		// 3. Migrate existing coordinator & principal data from SchoolDetails if columns exist, then drop from SchoolDetails
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'SchoolDetails' AND column_name = 'coordinatorName'
			) THEN
				INSERT INTO "CoordinatorDetails" (
					"id", "schoolId", "schoolName", "schoolCode", 
					"principalName", "coordinatorName", "coordinatorDesignation", "coordinatorMobile", "coordinatorEmail"
				)
				SELECT 
					gen_random_uuid()::text, "id", "schoolName", "schoolCode", 
					"principalName", "coordinatorName", "coordinatorDesignation", "coordinatorMobile", "coordinatorEmail"
				FROM "SchoolDetails"
				WHERE ("coordinatorName" IS NOT NULL OR "principalName" IS NOT NULL)
				  AND "id" NOT IN (SELECT "schoolId" FROM "CoordinatorDetails" WHERE "schoolId" IS NOT NULL);

				-- Remove decoupled coordinator & principal fields from SchoolDetails
				ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "principalName";
				ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorName";
				ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorDesignation";
				ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorMobile";
				ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorEmail";
			END IF;
		END $$;`,
	}

	for _, q := range queries {
		if _, err := rawDB.Exec(q); err != nil {
			log.Printf("Warning executing Coordinator schema query: %v", err)
		}
	}

	log.Println("CoordinatorDetails table verified and migrated successfully.")
}
