package dto

import "time"

// Technik Pride Award Nomination DTOs
type TechnikPrideNominationRequest struct {
	StudentName         string `json:"studentName" binding:"required"`
	Class               string `json:"class" binding:"required"`
	ClassCategory       string `json:"classCategory"`
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

// Student Roster DTOs
type SchoolStudentsResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Total    int               `json:"total"`
	Students []StudentResponse `json:"students"`
}

// Olympiad Registration DTOs
type OlympiadStudentDTO struct {
	StudentName   string `json:"studentName" binding:"required"`
	Class         string `json:"class" binding:"required"`
	ClassCategory string `json:"classCategory"`
	Gender        string `json:"gender" binding:"required"`
	OlympiadTrack string `json:"olympiadTrack" binding:"required"`
}

type OlympiadRegistrationRequest struct {
	SchoolID     string               `json:"schoolId"`
	AcademicYear int                  `json:"academicYear"`
	Students     []OlympiadStudentDTO `json:"students" binding:"required,gt=0"`
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
