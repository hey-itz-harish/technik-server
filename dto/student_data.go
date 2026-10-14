package dto

import "time"

// SaveStudentPortfolioRequest represents the request to update student text & file data
type SaveStudentPortfolioRequest struct {
	ProjectTitle       string   `json:"projectTitle"`
	ProjectDescription string   `json:"projectDescription"`
	StudentQuote       string   `json:"studentQuote"`
	KeyStrengths       string   `json:"keyStrengths"`
	DevelopmentAreas   string   `json:"developmentAreas"`
	MentorRemarks      string   `json:"mentorRemarks"`
	ProjectFile        string   `json:"projectFile"`
	StudentPhoto       string   `json:"studentPhoto"`
	AdditionalFiles    []string `json:"additionalFiles"`
}

// StudentPortfolioDTO represents the full data for a student's portfolio and Skill Compass
type StudentPortfolioDTO struct {
	StudentID          string    `json:"studentId"`
	StudentName        string    `json:"studentName"`
	Class              string    `json:"class"`
	SchoolName         string    `json:"schoolName"`
	Category           string    `json:"category"`
	AchievementTitle   string    `json:"achievementTitle"`
	ApprovalStatus     string    `json:"approvalStatus"`
	RegistrationNumber string    `json:"registrationNumber"`
	ProjectTitle       string    `json:"projectTitle"`
	ProjectDescription string    `json:"projectDescription"`
	StudentQuote       string    `json:"studentQuote"`
	KeyStrengths       string    `json:"keyStrengths"`
	DevelopmentAreas   string    `json:"developmentAreas"`
	MentorRemarks      string    `json:"mentorRemarks"`
	ProjectFile        string    `json:"projectFile"`
	StudentPhoto       string    `json:"studentPhoto"`
	AdditionalFiles    []string  `json:"additionalFiles"`
	AchievementScore   float64   `json:"achievementScore"`
	InnovationScore    float64   `json:"innovationScore"`
	CreativityScore    float64   `json:"creativityScore"`
	CommunicationScore float64   `json:"communicationScore"`
	LeadershipScore    float64   `json:"leadershipScore"`
	SocialImpactScore  float64   `json:"socialImpactScore"`
	PaymentStatus      string    `json:"paymentStatus"`
	AmountPaid         int       `json:"amountPaid"`
	PaidAt             *time.Time `json:"paidAt,omitempty"`
	RazorpayKeyID      string    `json:"razorpayKeyId,omitempty"`
}

// RazorpayCreateOrderRequest represents the request to create a Razorpay order
type RazorpayCreateOrderRequest struct {
	StudentID string `json:"studentId" binding:"required"`
	Amount    int    `json:"amount"` // in INR (e.g. 499)
}

// RazorpayCreateOrderResponse represents the response containing the Razorpay order ID
type RazorpayCreateOrderResponse struct {
	Success   bool   `json:"success"`
	OrderID   string `json:"orderId"`
	Amount    int    `json:"amount"` // in paise (e.g. 49900)
	Currency  string `json:"currency"`
	KeyID     string `json:"keyId"`
	StudentID string `json:"studentId"`
	StudentName string `json:"studentName"`
}

// RazorpayVerifyPaymentRequest represents the request to verify payment
type RazorpayVerifyPaymentRequest struct {
	StudentID         string `json:"studentId" binding:"required"`
	RazorpayOrderID   string `json:"razorpayOrderId" binding:"required"`
	RazorpayPaymentID string `json:"razorpayPaymentId" binding:"required"`
	RazorpaySignature string `json:"razorpaySignature" binding:"required"`
}
