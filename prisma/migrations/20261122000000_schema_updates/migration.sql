-- ====================================================================
-- TASK 1: Decouple Coordinator & Principal Details from SchoolDetails
-- ====================================================================

-- 1. Create CoordinatorDetails table
CREATE TABLE IF NOT EXISTS "CoordinatorDetails" (
    "id" TEXT NOT NULL,
    "schoolId" TEXT,
    "schoolName" TEXT,
    "schoolCode" TEXT,
    "principalName" TEXT,
    "coordinatorName" TEXT,
    "coordinatorDesignation" TEXT,
    "coordinatorMobile" TEXT,
    "coordinatorEmail" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deletedAt" TIMESTAMP(3),

    CONSTRAINT "CoordinatorDetails_pkey" PRIMARY KEY ("id")
);

-- Foreign Key to SchoolDetails
ALTER TABLE "CoordinatorDetails" 
    ADD CONSTRAINT "CoordinatorDetails_schoolId_fkey" 
    FOREIGN KEY ("schoolId") REFERENCES "SchoolDetails"("id") ON DELETE CASCADE ON UPDATE CASCADE;

CREATE INDEX IF NOT EXISTS "idx_coordinator_school_id" ON "CoordinatorDetails"("schoolId");
CREATE INDEX IF NOT EXISTS "idx_coordinator_school_code" ON "CoordinatorDetails"("schoolCode");

-- Migrate existing data from SchoolDetails into CoordinatorDetails if columns exist
DO $$
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

        -- Drop decoupled fields from SchoolDetails
        ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "principalName";
        ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorName";
        ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorDesignation";
        ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorMobile";
        ALTER TABLE "SchoolDetails" DROP COLUMN IF EXISTS "coordinatorEmail";
    END IF;
END $$;


-- ====================================================================
-- TASK 2: StudentDetails Schema Updates
-- ====================================================================

-- Remove parentName and add motherName & fatherName
ALTER TABLE "StudentDetails" DROP COLUMN IF EXISTS "parentName";
ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "fatherName" TEXT;
ALTER TABLE "StudentDetails" ADD COLUMN IF NOT EXISTS "motherName" TEXT;

-- Remove default 2026 for academicYear
ALTER TABLE "StudentDetails" ALTER COLUMN "academicYear" DROP DEFAULT;

-- Remove category, classCategory, status, assessment_grid, assessment_status
ALTER TABLE "StudentDetails" DROP COLUMN IF EXISTS "category";
ALTER TABLE "StudentDetails" DROP COLUMN IF EXISTS "classCategory";
ALTER TABLE "StudentDetails" DROP COLUMN IF EXISTS "status";
ALTER TABLE "StudentDetails" DROP COLUMN IF EXISTS "assessment_grid";
ALTER TABLE "StudentDetails" DROP COLUMN IF EXISTS "assessment_status";


-- ====================================================================
-- TASK 3: Add studentId to TechnikPrideNomination
-- ====================================================================

ALTER TABLE "TechnikPrideNomination" ADD COLUMN IF NOT EXISTS "studentId" TEXT;


-- ====================================================================
-- TASK 5: Change table name olympiad_student_allocations -> olympiadstudentteststatus
-- (Executed before Task 4 so foreign key references resolve properly)
-- ====================================================================

DO $$
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
END $$;

CREATE TABLE IF NOT EXISTS olympiadstudentteststatus (
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
);

CREATE INDEX IF NOT EXISTS idx_olympiad_status_test_id ON olympiadstudentteststatus(test_id);
CREATE INDEX IF NOT EXISTS idx_olympiad_status_code ON olympiadstudentteststatus(access_code);
CREATE INDEX IF NOT EXISTS idx_olympiad_status_email ON olympiadstudentteststatus(student_email);


-- ====================================================================
-- TASK 4: Remove OlympiadRegistration & OlympiadStudent, create olympiadParticipants
-- ====================================================================

DROP TABLE IF EXISTS "OlympiadStudent" CASCADE;
DROP TABLE IF EXISTS "OlympiadRegistration" CASCADE;

CREATE TABLE IF NOT EXISTS "olympiadParticipants" (
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
);

CREATE INDEX IF NOT EXISTS idx_olympiad_participants_test_id ON "olympiadParticipants"(test_id);
CREATE INDEX IF NOT EXISTS idx_olympiad_participants_student_id ON "olympiadParticipants"("studentId");
CREATE INDEX IF NOT EXISTS idx_olympiad_participants_school_code ON "olympiadParticipants"("schoolCode");
CREATE INDEX IF NOT EXISTS idx_olympiad_participants_status_id ON "olympiadParticipants"(olympiadstudentteststatusid);
