package dto

import "time"

// OlympiadQuestionDTO represents an individual exam question
type OlympiadQuestionDTO struct {
	ID             string   `json:"id,omitempty"`
	TestID         string   `json:"testId,omitempty"`
	QuestionNumber int      `json:"questionNumber"`
	QuestionText   string   `json:"questionText" binding:"required"`
	ImageURL       string   `json:"imageUrl,omitempty"`
	Options        []string `json:"options" binding:"required,min=2"`
	CorrectOption  int      `json:"correctOption"`
	Marks          int      `json:"marks"`
	Explanation    string   `json:"explanation,omitempty"`
}

// OlympiadTestResponse represents an Olympiad Test record
type OlympiadTestResponse struct {
	ID                  string                `json:"id"`
	Title               string                `json:"title"`
	Track               string                `json:"track"`
	ClassCategory       string                `json:"classCategory"`
	StartDate           string                `json:"startDate"`
	EndDate             string                `json:"endDate"`
	StartTime           string                `json:"startTime"`
	EndTime             string                `json:"endTime"`
	DurationMinutes     int                   `json:"durationMinutes"`
	TotalMarks          int                   `json:"totalMarks"`
	PassingMarks        int                   `json:"passingMarks"`
	Instructions        string                `json:"instructions"`
	IsActive            bool                  `json:"isActive"`
	TotalQuestions      int                   `json:"totalQuestions"`
	TotalAllocated      int                   `json:"totalAllocated,omitempty"`
	Questions           []OlympiadQuestionDTO `json:"questions,omitempty"`
	CertificateTemplate string                `json:"certificateTemplate,omitempty"`
	CreatedAt           time.Time             `json:"createdAt"`
	UpdatedAt           time.Time             `json:"updatedAt"`
}

// CreateOlympiadTestRequest is payload for creating/editing a test
type CreateOlympiadTestRequest struct {
	Title           string                `json:"title" binding:"required"`
	Track           string                `json:"track" binding:"required"`
	ClassCategory   string                `json:"classCategory" binding:"required"`
	StartDate       string                `json:"startDate" binding:"required"`
	EndDate         string                `json:"endDate" binding:"required"`
	StartTime       string                `json:"startTime" binding:"required"`
	EndTime         string                `json:"endTime" binding:"required"`
	DurationMinutes int                   `json:"durationMinutes" binding:"required,min=5"`
	TotalMarks      int                   `json:"totalMarks"`
	PassingMarks    int                   `json:"passingMarks"`
	Instructions    string                `json:"instructions"`
	IsActive        bool                  `json:"isActive"`
	Questions       []OlympiadQuestionDTO `json:"questions"`
}

// BulkUploadQuestionsRequest is used for bulk uploading questions
type BulkUploadQuestionsRequest struct {
	Questions []OlympiadQuestionDTO `json:"questions" binding:"required"`
}

// StudentAllocationResponse represents an allocated student record
type StudentAllocationResponse struct {
	ID                string                `json:"id"`
	TestID            string                `json:"testId"`
	TestTitle         string                `json:"testTitle,omitempty"`
	StudentName       string                `json:"studentName"`
	StudentEmail      string                `json:"studentEmail"`
	StudentPhone      string                `json:"studentPhone"`
	SchoolName        string                `json:"schoolName"`
	Grade             string                `json:"grade"`
	AccessCode        string                `json:"accessCode"`
	ScheduledDate     string                `json:"scheduledDate"`
	AllocatedDuration int                   `json:"allocatedDuration"`
	TimeRemaining     int                   `json:"timeRemaining"`
	Status            string                `json:"status"`
	ResumeAllowed     bool                  `json:"resumeAllowed"`
	CurrentQuestion   int                   `json:"currentQuestion"`
	Answers           map[string]int        `json:"answers"`
	ProctorLogs       []ProctorViolationLog `json:"proctorLogs"`
	ScreenShareActive bool                  `json:"screenShareActive"`
	ScreenSnapshot    string                `json:"screenSnapshot,omitempty"`
	Blocked           bool                  `json:"blocked"`
	BlockedReason     string                `json:"blockedReason,omitempty"`
	AutoResume        bool                  `json:"autoResume"`
	StartedAt         *time.Time            `json:"startedAt,omitempty"`
	LastHeartbeatAt   *time.Time            `json:"lastHeartbeatAt,omitempty"`
	CompletedAt       *time.Time            `json:"completedAt,omitempty"`
	Score             int                   `json:"score"`
	TotalMarks        int                   `json:"totalMarks,omitempty"`
	TotalQuestions    int                   `json:"totalQuestions"`
	CertificateIssued bool                  `json:"certificateIssued"`
	CertificateURL    string                `json:"certificateUrl,omitempty"`
	AdminNotes        string                `json:"adminNotes"`
	CreatedAt         time.Time             `json:"createdAt"`
	UpdatedAt         time.Time             `json:"updatedAt"`
}

// AllocateStudentRequest is payload for allocating a student
type AllocateStudentRequest struct {
	StudentName       string `json:"studentName" binding:"required"`
	StudentEmail      string `json:"studentEmail" binding:"required,email"`
	StudentPhone      string `json:"studentPhone"`
	SchoolName        string `json:"schoolName"`
	Grade             string `json:"grade"`
	ScheduledDate     string `json:"scheduledDate"`
	AllocatedDuration int    `json:"allocatedDuration"`
}

// BulkAllocateStudentsRequest is for bulk assigning students
type BulkAllocateStudentsRequest struct {
	Students []AllocateStudentRequest `json:"students" binding:"required"`
}

// EditStudentTimeRequest allows admin to add or set remaining time
type EditStudentTimeRequest struct {
	AdditionalMinutes     int    `json:"additionalMinutes"`
	ExactRemainingMinutes int    `json:"exactRemainingMinutes"`
	AdminNotes            string `json:"adminNotes"`
}

// ResumeStudentTestRequest allows admin to enable test resume
type ResumeStudentTestRequest struct {
	ResumeAllowed bool   `json:"resumeAllowed"`
	AdminNotes    string `json:"adminNotes"`
}

// ResetStudentTestRequest completely restarts test session
type ResetStudentTestRequest struct {
	AdminNotes string `json:"adminNotes"`
}

// ProctorViolationLog represents an anti-cheating / proctor alert
type ProctorViolationLog struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"eventType"`
	Details   string `json:"details"`
}

// StudentExamVerifyRequest is sent when student enters access code
type StudentExamVerifyRequest struct {
	AccessCode string `json:"accessCode" form:"accessCode"`
}

// StudentExamVerifyResponse provides test info and candidate attempt state
type StudentExamVerifyResponse struct {
	Success           bool                  `json:"success"`
	Message           string                `json:"message"`
	AllocationID      string                `json:"allocationId"`
	TestID            string                `json:"testId"`
	TestTitle         string                `json:"testTitle"`
	Track             string                `json:"track"`
	ClassCategory     string                `json:"classCategory"`
	Instructions      string                `json:"instructions"`
	StartDate         string                `json:"startDate"`
	EndDate           string                `json:"endDate"`
	StartTime         string                `json:"startTime"`
	EndTime           string                `json:"endTime"`
	StudentName       string                `json:"studentName"`
	StudentEmail      string                `json:"studentEmail"`
	SchoolName        string                `json:"schoolName"`
	Grade             string                `json:"grade"`
	AccessCode        string                `json:"accessCode"`
	Status            string                `json:"status"`
	ResumeAllowed     bool                  `json:"resumeAllowed"`
	Blocked           bool                  `json:"blocked"`
	BlockedReason     string                `json:"blockedReason,omitempty"`
	AutoResume        bool                  `json:"autoResume"`
	AllocatedDuration int                   `json:"allocatedDuration"`
	TimeRemaining     int                   `json:"timeRemaining"`
	TotalQuestions    int                   `json:"totalQuestions"`
	CurrentQuestion   int                   `json:"currentQuestion"`
	SavedAnswers      map[string]int        `json:"savedAnswers"`
	Questions         []OlympiadQuestionDTO `json:"questions,omitempty"`
}

// StudentExamStartRequest starts or resumes the live session
type StudentExamStartRequest struct {
	AccessCode        string `json:"accessCode" binding:"required"`
	ScreenShareActive bool   `json:"screenShareActive"`
}

// StudentExamHeartbeatRequest sent periodically to sync state
type StudentExamHeartbeatRequest struct {
	AccessCode        string                `json:"accessCode" binding:"required"`
	TimeRemaining     int                   `json:"timeRemaining"`
	CurrentQuestion   int                   `json:"currentQuestion"`
	Answers           map[string]int        `json:"answers"`
	ScreenShareActive bool                  `json:"screenShareActive"`
	ScreenSnapshot    string                `json:"screenSnapshot,omitempty"`
	Blocked           bool                  `json:"blocked,omitempty"`
	BlockedReason     string                `json:"blockedReason,omitempty"`
	Violations        []ProctorViolationLog `json:"violations"`
}

// StudentExamHeartbeatResponse returns server state update
type StudentExamHeartbeatResponse struct {
	Success       bool   `json:"success"`
	Status        string `json:"status"`
	TimeRemaining int    `json:"timeRemaining"`
	ResumeAllowed bool   `json:"resumeAllowed"`
	Blocked       bool   `json:"blocked,omitempty"`
	Message       string `json:"message,omitempty"`
}

// StudentExamInterruptRequest records accidental closure for auto-resume
type StudentExamInterruptRequest struct {
	AccessCode      string         `json:"accessCode" binding:"required"`
	TimeRemaining   int            `json:"timeRemaining"`
	CurrentQuestion int            `json:"currentQuestion"`
	Answers         map[string]int `json:"answers"`
	Reason          string         `json:"reason"`
}

// StudentExamBlockRequest blocks the student when tab switch occurs
type StudentExamBlockRequest struct {
	AccessCode string `json:"accessCode" binding:"required"`
	Reason     string `json:"reason" binding:"required"`
	Details    string `json:"details"`
}

// StudentSaveAnswerRequest saves an answer
type StudentSaveAnswerRequest struct {
	AccessCode     string `json:"accessCode" binding:"required"`
	QuestionNumber int    `json:"questionNumber"`
	SelectedOption int    `json:"selectedOption"`
	TimeRemaining  int    `json:"timeRemaining"`
}

// StudentLogEventRequest records a proctoring infraction
type StudentLogEventRequest struct {
	AccessCode string              `json:"accessCode" binding:"required"`
	Event      ProctorViolationLog `json:"event" binding:"required"`
}

// StudentSubmitExamRequest finalizes the exam
type StudentSubmitExamRequest struct {
	AccessCode string         `json:"accessCode" binding:"required"`
	Answers    map[string]int `json:"answers"`
}

// StudentSubmitExamResponse gives confirmation and score summary
type StudentSubmitExamResponse struct {
	Success             bool      `json:"success"`
	Message             string    `json:"message"`
	SubmissionID        string    `json:"submissionId"`
	StudentName         string    `json:"studentName"`
	SchoolName          string    `json:"schoolName"`
	Grade               string    `json:"grade"`
	AccessCode          string    `json:"accessCode"`
	TotalQuestions      int       `json:"totalQuestions"`
	AttemptedCount      int       `json:"attemptedCount"`
	Score               int       `json:"score"`
	TotalMarks          int       `json:"totalMarks"`
	PassingMarks        int       `json:"passingMarks"`
	Passed              bool      `json:"passed"`
	CertificateTemplate string    `json:"certificateTemplate,omitempty"`
	CompletedAt         time.Time `json:"completedAt"`
}

// ExamResultRequest is used to look up results by access code
type ExamResultRequest struct {
	AccessCode string `json:"accessCode" form:"accessCode"`
}

// ExamResultResponse returns result and certificate for a submitted exam
type ExamResultResponse struct {
	Success             bool      `json:"success"`
	StudentName         string    `json:"studentName"`
	StudentEmail        string    `json:"studentEmail"`
	SchoolName          string    `json:"schoolName"`
	Grade               string    `json:"grade"`
	AccessCode          string    `json:"accessCode"`
	TestTitle           string    `json:"testTitle"`
	Track               string    `json:"track"`
	Score               int       `json:"score"`
	TotalMarks          int       `json:"totalMarks"`
	PassingMarks        int       `json:"passingMarks"`
	AttemptedCount      int       `json:"attemptedCount"`
	TotalQuestions      int       `json:"totalQuestions"`
	Passed              bool      `json:"passed"`
	SubmissionID        string    `json:"submissionId"`
	CertificateTemplate string    `json:"certificateTemplate,omitempty"`
	CertificateIssued   bool      `json:"certificateIssued"`
	CompletedAt         time.Time `json:"completedAt"`
}

// UploadCertificateTemplateRequest holds the certificate background image
type UploadCertificateTemplateRequest struct {
	TestID              string `json:"testId" binding:"required"`
	CertificateTemplate string `json:"certificateTemplate" binding:"required"`
}

// IssueCertificateRequest admin can manually issue/revoke certificates
type IssueCertificateRequest struct {
	AllocationID      string `json:"allocationId" binding:"required"`
	CertificateIssued bool   `json:"certificateIssued"`
}
