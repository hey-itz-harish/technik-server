package dto

// AssessmentQuestion represents an individual question extracted from the paper
type AssessmentQuestion struct {
	QuestionNumber        int     `json:"question_number"`
	QuestionText          string  `json:"question_text"`
	QuestionAllocatedMark float64 `json:"question_allocated_mark"`
}

// AssessmentSession represents a Roman numeral session (e.g. I, II, III)
type AssessmentSession struct {
	SessionNo            string               `json:"session_no"`
	SessionTitle         string               `json:"session_title,omitempty"`
	SessionAllocatedMark float64              `json:"session_allocated_mark"`
	Questions            []AssessmentQuestion `json:"questions"`
}

// ScrapedAssessmentData represents the parsed structure extracted by OpenAI / Document Parser
type ScrapedAssessmentData struct {
	NumberOfQuestions int                 `json:"number_of_questions"`
	TotalMark         float64             `json:"total_mark"`
	RomanNumerals     []string            `json:"roman_numerals"`
	Sessions          []AssessmentSession `json:"sessions"`
}

// GridRowDTO has the 6/7 core properties required for the student evaluation answer sheet grid
type GridRowDTO struct {
	SessionNo             string  `json:"session_no"`
	QuestionNumber        int     `json:"question_number"`
	SessionAllocatedMark  float64 `json:"session_allocated_mark"`
	QuestionAllocatedMark float64 `json:"question_allocated_mark"`
	AnswerMark            float64 `json:"answer_mark"`
	ExamName              string  `json:"exam_name"`
	ExamDate              string  `json:"exam_date"`
}

// AcademicAssessmentDTO represents the academic assessment database model
type AcademicAssessmentDTO struct {
	ID                              string              `json:"id"`
	SchoolName                      string              `json:"school_name"`
	SchoolID                        string              `json:"school_id"`
	ExamName                        string              `json:"exam_name"`
	ExamQuestionPaperDocumentPath  string              `json:"exam_question_paper_document_path"`
	Class                           string              `json:"class"`
	ExamDate                        string              `json:"exam_date"`
	NumberOfQuestions               int                 `json:"number_of_questions"`
	TotalMark                       float64             `json:"total_mark"`
	Questions                       []AssessmentSession `json:"questions"`
	TotalStudents                   int                 `json:"total_students,omitempty"`
	EvaluatedStudents               int                 `json:"evaluated_students,omitempty"`
	PromotedStudents                int                 `json:"promoted_students,omitempty"`
	DebarredStudents                int                 `json:"debarred_students,omitempty"`
	CreatedAt                       string              `json:"created_at"`
	CreatedBy                       string              `json:"created_by"`
	UpdatedAt                       string              `json:"updated_at"`
	UpdatedBy                       string              `json:"updated_by"`
	DeletedAt                       *string             `json:"deleted_at,omitempty"`
	DeletedBy                       string              `json:"deleted_by,omitempty"`
}

// StudentAcademicEvaluationDTO represents a mapped student's evaluation record with their answer sheet grid
type StudentAcademicEvaluationDTO struct {
	ID             string       `json:"id"`
	AssessmentID   string       `json:"assessment_id"`
	StudentID      string       `json:"student_id"`
	StudentName    string       `json:"student_name"`
	SchoolID       string       `json:"school_id"`
	SchoolName     string       `json:"school_name"`
	Class          string       `json:"class"`
	ExamName       string       `json:"exam_name"`
	ExamDate       string       `json:"exam_date"`
	GridData       []GridRowDTO `json:"grid_data"`
	TotalMarks     float64      `json:"total_marks"`
	ObtainedMarks  float64      `json:"obtained_marks"`
	Percentage     float64      `json:"percentage"`
	Status         string       `json:"status"` // "Pending", "Evaluated", "Promoted", "Debarred"
	DebarReason    string       `json:"debar_reason,omitempty"`
	EvaluatorNotes string       `json:"evaluator_notes,omitempty"`
	CreatedAt      string       `json:"created_at"`
	CreatedBy      string       `json:"created_by"`
	UpdatedAt      string       `json:"updated_at"`
	UpdatedBy      string       `json:"updated_by"`
}

// EvaluateStudentRequest receives answer sheet marks for a student
type EvaluateStudentRequest struct {
	StudentID      string       `json:"student_id"`
	GridData       []GridRowDTO `json:"grid_data"`
	EvaluatorNotes string       `json:"evaluator_notes"`
}

// PromoteStudentRequest handles student promotion
type PromoteStudentRequest struct {
	StudentID        string `json:"student_id"`
	PromoteNextGrade bool   `json:"promote_next_grade"`
	NextGrade        string `json:"next_grade,omitempty"`
	Remarks          string `json:"remarks"`
}

// DebarStudentRequest handles student debarment
type DebarStudentRequest struct {
	StudentID   string `json:"student_id"`
	DebarReason string `json:"debar_reason"`
}

// BatchActionRequest handles batch promote or debar
type BatchActionRequest struct {
	StudentIDs  []string `json:"student_ids"`
	Action      string   `json:"action"` // "promote" or "debar"
	DebarReason string   `json:"debar_reason,omitempty"`
	NextGrade   string   `json:"next_grade,omitempty"`
}

