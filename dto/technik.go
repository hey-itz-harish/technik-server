package dto

import "time"

// Technik Pride Award Nomination DTOs
type TechnikPrideNominationRequest struct {
	StudentID           string `json:"studentId,omitempty"`
	StudentName         string `json:"studentName" binding:"required"`
	Class               string `json:"class" binding:"required"`
	ClassCategory       string `json:"classCategory,omitempty"`
	Gender              string `json:"gender" binding:"required"`
	AchievementCategory string `json:"achievementCategory" binding:"required"`
	AchievementTitle    string `json:"achievementTitle" binding:"required"`
	BriefDescription    string `json:"briefDescription" binding:"required,max=300"`
	SupportingDocument  string `json:"supportingDocument"`
	AcademicYear        int    `json:"academicYear"`
	SchoolID            string `json:"schoolId"`
}

type TechnikPrideBatchNominationRequest struct {
	SchoolID     string                          `json:"schoolId"`
	AcademicYear int                             `json:"academicYear"`
	Nominations  []TechnikPrideNominationRequest `json:"nominations" binding:"required,gt=0"`
}

type TechnikPrideNominationResponse struct {
	ID                  string    `json:"id"`
	SchoolID            string    `json:"schoolId,omitempty"`
	StudentID           string    `json:"studentId,omitempty"`
	AcademicYear        int       `json:"academicYear"`
	StudentName         string    `json:"studentName"`
	Class               string    `json:"class"`
	ClassCategory       string    `json:"classCategory,omitempty"`
	Gender              string    `json:"gender"`
	AchievementCategory string    `json:"achievementCategory"`
	AchievementTitle    string    `json:"achievementTitle"`
	BriefDescription    string    `json:"briefDescription"`
	SupportingDocument  string    `json:"supportingDocument,omitempty"`
	NominationStatus    string    `json:"nominationStatus"`
	AdminStatus         string    `json:"adminStatus"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type TechnikPrideAuthResponse struct {
	Success     bool                             `json:"success"`
	Message     string                           `json:"message"`
	Total       int                              `json:"total,omitempty"`
	Nominations []TechnikPrideNominationResponse `json:"nominations,omitempty"`
}

// Student Roster & Management DTOs
type SchoolStudentsResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Total    int               `json:"total"`
	Students []StudentResponse `json:"students"`
}

type CreateStudentRequest struct {
	StudentName  string `json:"studentName" binding:"required"`
	Grade        string `json:"grade" binding:"required"`
	Section      string `json:"section,omitempty"`
	RollNo       string `json:"rollNo,omitempty"`
	Gender       string `json:"gender,omitempty"`
	FatherName   string `json:"fatherName,omitempty"`
	MotherName   string `json:"motherName,omitempty"`
	ParentPhone  string `json:"parentPhone,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	Address      string `json:"address,omitempty"`
	AcademicYear int    `json:"academicYear,omitempty"`
	SchoolID     string `json:"schoolId,omitempty"`
}

type UpdateStudentRequest struct {
	StudentName  string `json:"studentName,omitempty"`
	Grade        string `json:"grade,omitempty"`
	Section      string `json:"section,omitempty"`
	RollNo       string `json:"rollNo,omitempty"`
	Gender       string `json:"gender,omitempty"`
	FatherName   string `json:"fatherName,omitempty"`
	MotherName   string `json:"motherName,omitempty"`
	ParentPhone  string `json:"parentPhone,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	Address      string `json:"address,omitempty"`
	AcademicYear int    `json:"academicYear,omitempty"`
}

type BulkDeleteStudentsRequest struct {
	StudentIDs []string `json:"studentIds" binding:"required,gt=0"`
}

type StudentExcelRow struct {
	StudentName  string `json:"studentName"`
	Grade        string `json:"grade"`
	Section      string `json:"section"`
	RollNo       string `json:"rollNo"`
	Gender       string `json:"gender"`
	FatherName   string `json:"fatherName"`
	MotherName   string `json:"motherName"`
	ParentPhone  string `json:"parentPhone"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Address      string `json:"address"`
	AcademicYear int    `json:"academicYear"`
}

type StudentExcelImportResponse struct {
	Success       bool              `json:"success"`
	Message       string            `json:"message"`
	TotalRows     int               `json:"totalRows"`
	ImportedCount int               `json:"importedCount"`
	UpdatedCount  int               `json:"updatedCount"`
	FailedCount   int               `json:"failedCount"`
	Errors        []string          `json:"errors,omitempty"`
	Students      []StudentResponse `json:"students,omitempty"`
}

// Olympiad Participant DTOs
type OlympiadStudentDTO struct {
	StudentID     string `json:"studentId,omitempty"`
	StudentName   string `json:"studentName" binding:"required"`
	Class         string `json:"class" binding:"required"`
	ClassCategory string `json:"classCategory,omitempty"`
	Gender        string `json:"gender" binding:"required"`
	OlympiadTrack string `json:"olympiadTrack" binding:"required"`
	TestID        string `json:"testId,omitempty"`
}

type OlympiadRegistrationRequest struct {
	SchoolID     string               `json:"schoolId"`
	AcademicYear int                  `json:"academicYear"`
	TestID       string               `json:"testId,omitempty"`
	Students     []OlympiadStudentDTO `json:"students" binding:"required,gt=0"`
}

type DirectStudentOlympiadRegisterRequest struct {
	TestID string `json:"testId" binding:"required"`
}

type OlympiadParticipantResponse struct {
	ID                          string     `json:"id"`
	StudentName                 string     `json:"studentName"`
	StudentID                   string     `json:"studentId,omitempty"`
	SchoolName                  string     `json:"schoolName,omitempty"`
	SchoolCode                  string     `json:"schoolCode,omitempty"`
	Gender                      string     `json:"gender"`
	OlympiadTrack               string     `json:"olympiadTrack"`
	TestID                      string     `json:"testId,omitempty"`
	TestTitle                   string     `json:"testTitle,omitempty"`
	OlympiadQuestionsID         string     `json:"olympiadQuestionsId,omitempty"`
	OlympiadStudentTestStatusID string     `json:"olympiadStudentTestStatusId,omitempty"`
	TestStatus                  string     `json:"testStatus,omitempty"`
	Score                       int        `json:"score,omitempty"`
	TotalMarks                  int        `json:"totalMarks,omitempty"`
	CertificateIssued           bool       `json:"certificateIssued"`
	CertificateURL              string     `json:"certificateUrl,omitempty"`
	CreatedAt                   time.Time  `json:"createdAt"`
	CreatedBy                   string     `json:"createdBy,omitempty"`
	UpdatedAt                   time.Time  `json:"updatedAt"`
	Deleted                     bool       `json:"deleted"`
	DeletedAt                   *time.Time `json:"deletedAt,omitempty"`
}

type OlympiadStudentResponse struct {
	ID            string    `json:"id"`
	StudentName   string    `json:"studentName"`
	Class         string    `json:"class"`
	ClassCategory string    `json:"classCategory,omitempty"`
	Gender        string    `json:"gender"`
	OlympiadTrack string    `json:"olympiadTrack"`
	CreatedAt     time.Time `json:"createdAt"`
}

type OlympiadRegistrationResponse struct {
	ID            string                    `json:"id"`
	SchoolID      string                    `json:"schoolId,omitempty"`
	AcademicYear  int                       `json:"academicYear"`
	TotalStudents int                       `json:"totalStudents"`
	Students      []OlympiadStudentResponse `json:"students"`
	CreatedAt     time.Time                 `json:"createdAt"`
}

type OlympiadBatchResponse struct {
	Success      bool                         `json:"success"`
	Message      string                       `json:"message"`
	Registration OlympiadRegistrationResponse `json:"registration"`
}

type BatchEvaluateRequest struct {
	BatchSize int `json:"batchSize,omitempty"` // Default 100
}

type BatchEvaluateResponse struct {
	Success            bool   `json:"success"`
	Message            string `json:"message"`
	TestID             string `json:"testId"`
	BatchSize          int    `json:"batchSize"`
	EvaluatedCount     int    `json:"evaluatedCount"`
	PassedCount        int    `json:"passedCount"`
	FailedCount        int    `json:"failedCount"`
	RemainingUnscored  int    `json:"remainingUnscored"`
}

type CertificateGenerationResponse struct {
	Success        bool     `json:"success"`
	Message        string   `json:"message"`
	TestID         string   `json:"testId"`
	GeneratedCount int      `json:"generatedCount"`
	EmailedCount   int      `json:"emailedCount"`
	CertificateURLs []string `json:"certificateUrls,omitempty"`
}

type QuestionReminderResponse struct {
	TestID         string `json:"testId"`
	TestTitle      string `json:"testTitle"`
	Track          string `json:"track"`
	StartDate      string `json:"startDate"`
	DaysRemaining  int    `json:"daysRemaining"`
	TotalQuestions int    `json:"totalQuestions"`
	NeedsQuestions bool   `json:"needsQuestions"`
	AlertMessage   string `json:"alertMessage"`
}
