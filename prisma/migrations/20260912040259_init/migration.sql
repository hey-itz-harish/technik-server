-- CreateEnum
CREATE TYPE "Role" AS ENUM ('ADMIN', 'USER');

-- CreateTable
CREATE TABLE "SchoolDetails" (
    "id" TEXT NOT NULL,
    "schoolCode" TEXT NOT NULL,
    "schoolName" TEXT NOT NULL,
    "board" TEXT,
    "state" TEXT,
    "district" TEXT,
    "city" TEXT,
    "address" TEXT,
    "pincode" TEXT,
    "email" TEXT NOT NULL,
    "phone" TEXT,
    "password" TEXT NOT NULL,
    "principalName" TEXT,
    "coordinatorName" TEXT,
    "coordinatorDesignation" TEXT,
    "coordinatorMobile" TEXT,
    "coordinatorEmail" TEXT,
    "isActivated" BOOLEAN NOT NULL DEFAULT false,
    "isVerified" BOOLEAN NOT NULL DEFAULT false,
    "activationToken" TEXT,
    "activationExpiry" TIMESTAMP(3),
    "otp" TEXT,
    "otpExpiry" TIMESTAMP(3),
    "msAuthSecret" TEXT,
    "twoFactorEnable" BOOLEAN NOT NULL DEFAULT false,
    "csrfID" TEXT,
    "sessionId" TEXT,
    "ipAddress" TEXT,
    "sessionExpiry" TIMESTAMP(3),
    "rememberMe" BOOLEAN NOT NULL DEFAULT false,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    "deletedAt" TIMESTAMP(3),

    CONSTRAINT "SchoolDetails_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "StudentDetails" (
    "id" TEXT NOT NULL,
    "studentName" TEXT NOT NULL,
    "email" TEXT,
    "password" TEXT,
    "phone" TEXT,
    "grade" TEXT NOT NULL,
    "section" TEXT,
    "rollNo" TEXT,
    "parentName" TEXT,
    "parentPhone" TEXT,
    "address" TEXT,
    "schoolId" TEXT,
    "isActivated" BOOLEAN NOT NULL DEFAULT false,
    "isVerified" BOOLEAN NOT NULL DEFAULT false,
    "activationToken" TEXT,
    "activationExpiry" TIMESTAMP(3),
    "otp" TEXT,
    "otpExpiry" TIMESTAMP(3),
    "msAuthSecret" TEXT,
    "twoFactorEnable" BOOLEAN NOT NULL DEFAULT false,
    "csrfID" TEXT,
    "sessionId" TEXT,
    "ipAddress" TEXT,
    "sessionExpiry" TIMESTAMP(3),
    "rememberMe" BOOLEAN NOT NULL DEFAULT false,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    "deletedAt" TIMESTAMP(3),

    CONSTRAINT "StudentDetails_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "User" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "email" TEXT NOT NULL,
    "password" TEXT NOT NULL,
    "role" "Role" NOT NULL DEFAULT 'USER',
    "isActivated" BOOLEAN NOT NULL DEFAULT false,
    "isVerified" BOOLEAN NOT NULL DEFAULT false,
    "activationToken" TEXT,
    "activationExpiry" TIMESTAMP(3),
    "otp" TEXT,
    "otpExpiry" TIMESTAMP(3),
    "msAuthSecret" TEXT,
    "twoFactorEnable" BOOLEAN NOT NULL DEFAULT false,
    "csrfID" TEXT,
    "sessionId" TEXT,
    "ipAddress" TEXT,
    "sessionExpiry" TIMESTAMP(3),
    "rememberMe" BOOLEAN NOT NULL DEFAULT false,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    "deletedAt" TIMESTAMP(3),

    CONSTRAINT "User_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "AuditLog" (
    "id" TEXT NOT NULL,
    "method" TEXT NOT NULL,
    "path" TEXT NOT NULL,
    "userEmail" TEXT,
    "ipAddress" TEXT,
    "userAgent" TEXT,
    "statusCode" INTEGER NOT NULL,
    "latencyMs" INTEGER NOT NULL,
    "requestBody" TEXT,
    "responseBody" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "AuditLog_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "SessionLog" (
    "id" TEXT NOT NULL,
    "userEmail" TEXT NOT NULL,
    "sessionId" TEXT,
    "ipAddress" TEXT,
    "userAgent" TEXT,
    "action" TEXT NOT NULL,
    "isActive" BOOLEAN NOT NULL DEFAULT true,
    "loginAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "logoutAt" TIMESTAMP(3),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "SessionLog_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "TechnikPrideNomination" (
    "id" TEXT NOT NULL,
    "schoolId" TEXT,
    "academicYear" INTEGER NOT NULL DEFAULT 2026,
    "studentName" TEXT NOT NULL,
    "class" TEXT NOT NULL,
    "classCategory" TEXT,
    "gender" TEXT NOT NULL,
    "achievementCategory" TEXT NOT NULL,
    "achievementTitle" TEXT NOT NULL,
    "briefDescription" TEXT NOT NULL,
    "supportingDocument" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "TechnikPrideNomination_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "OlympiadRegistration" (
    "id" TEXT NOT NULL,
    "schoolId" TEXT,
    "academicYear" INTEGER NOT NULL DEFAULT 2026,
    "totalStudents" INTEGER NOT NULL DEFAULT 0,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "OlympiadRegistration_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "OlympiadStudent" (
    "id" TEXT NOT NULL,
    "olympiadRegistrationId" TEXT NOT NULL,
    "studentName" TEXT NOT NULL,
    "class" TEXT NOT NULL,
    "classCategory" TEXT,
    "gender" TEXT NOT NULL,
    "olympiadTrack" TEXT NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "OlympiadStudent_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "SchoolDetails_schoolCode_key" ON "SchoolDetails"("schoolCode");

-- CreateIndex
CREATE UNIQUE INDEX "SchoolDetails_email_key" ON "SchoolDetails"("email");

-- CreateIndex
CREATE UNIQUE INDEX "StudentDetails_email_key" ON "StudentDetails"("email");

-- CreateIndex
CREATE UNIQUE INDEX "User_email_key" ON "User"("email");

-- AddForeignKey
ALTER TABLE "StudentDetails" ADD CONSTRAINT "StudentDetails_schoolId_fkey" FOREIGN KEY ("schoolId") REFERENCES "SchoolDetails"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "TechnikPrideNomination" ADD CONSTRAINT "TechnikPrideNomination_schoolId_fkey" FOREIGN KEY ("schoolId") REFERENCES "SchoolDetails"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "OlympiadRegistration" ADD CONSTRAINT "OlympiadRegistration_schoolId_fkey" FOREIGN KEY ("schoolId") REFERENCES "SchoolDetails"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "OlympiadStudent" ADD CONSTRAINT "OlympiadStudent_olympiadRegistrationId_fkey" FOREIGN KEY ("olympiadRegistrationId") REFERENCES "OlympiadRegistration"("id") ON DELETE CASCADE ON UPDATE CASCADE;
