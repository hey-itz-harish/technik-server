package dto

import "time"

// School DTOs
type SchoolRegisterRequest struct {
	SchoolName             string `json:"schoolName" binding:"required,min=2"`
	Board                  string `json:"board"`
	State                  string `json:"state"`
	District               string `json:"district"`
	City                   string `json:"city"`
	Address                string `json:"address"`
	Pincode                string `json:"pincode"`
	Email                  string `json:"email" binding:"required,email"`
	Phone                  string `json:"phone"`
	SchoolMobile           string `json:"schoolMobile"`
	Password               string `json:"password" binding:"required,min=6"`
	PrincipalName          string `json:"principalName"`
	CoordinatorName        string `json:"coordinatorName"`
	CoordinatorDesignation string `json:"coordinatorDesignation"`
	CoordinatorMobile      string `json:"coordinatorMobile"`
	CoordinatorEmail       string `json:"coordinatorEmail"`
}

type SchoolLoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

type SchoolResponse struct {
	ID                     string     `json:"id"`
	SchoolCode             string     `json:"schoolCode"`
	SchoolName             string     `json:"schoolName"`
	Board                  string     `json:"board,omitempty"`
	State                  string     `json:"state,omitempty"`
	District               string     `json:"district,omitempty"`
	City                   string     `json:"city,omitempty"`
	Address                string     `json:"address,omitempty"`
	Pincode                string     `json:"pincode,omitempty"`
	Email                  string     `json:"email"`
	Phone                  string     `json:"phone,omitempty"`
	PrincipalName          string     `json:"principalName,omitempty"`
	CoordinatorName        string     `json:"coordinatorName,omitempty"`
	CoordinatorDesignation string     `json:"coordinatorDesignation,omitempty"`
	CoordinatorMobile      string     `json:"coordinatorMobile,omitempty"`
	CoordinatorEmail       string     `json:"coordinatorEmail,omitempty"`
	TwoFactorEnable        bool       `json:"twoFactorEnable"`
	IsActivated            bool       `json:"isActivated"`
	IsVerified             bool       `json:"isVerified"`
	CSRFID                 string     `json:"csrfID,omitempty"`
	SessionID              string     `json:"sessionId,omitempty"`
	IPAddress              string     `json:"ipAddress,omitempty"`
	RememberMe             bool       `json:"rememberMe"`
	SessionExpiry          *time.Time `json:"sessionExpiry,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

// Student DTOs
type StudentRegisterRequest struct {
	StudentName string `json:"studentName" binding:"required,min=2"`
	Email       string `json:"email" binding:"omitempty,email"`
	Password    string `json:"password" binding:"omitempty,min=6"`
	Phone       string `json:"phone"`
	Grade       string `json:"grade" binding:"required"`
	Section     string `json:"section"`
	RollNo      string `json:"rollNo"`
	ParentName  string `json:"parentName"`
	ParentPhone string `json:"parentPhone"`
	Address     string `json:"address"`
	SchoolID    string `json:"schoolId"`
}

type StudentLoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

type StudentResponse struct {
	ID              string          `json:"id"`
	StudentName     string          `json:"studentName"`
	Email           string          `json:"email,omitempty"`
	Phone           string          `json:"phone,omitempty"`
	Grade           string          `json:"grade"`
	Section         string          `json:"section,omitempty"`
	RollNo          string          `json:"rollNo,omitempty"`
	ParentName      string          `json:"parentName,omitempty"`
	ParentPhone     string          `json:"parentPhone,omitempty"`
	School          *SchoolResponse `json:"school,omitempty"`
	TwoFactorEnable bool            `json:"twoFactorEnable"`
	IsActivated     bool            `json:"isActivated"`
	IsVerified      bool            `json:"isVerified"`
	CSRFID          string          `json:"csrfID,omitempty"`
	SessionID       string          `json:"sessionId,omitempty"`
	IPAddress       string          `json:"ipAddress,omitempty"`
	RememberMe      bool            `json:"rememberMe"`
	SessionExpiry   *time.Time      `json:"sessionExpiry,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// Activation DTOs
type ActivateAccountRequest struct {
	Token string `json:"token" form:"token" binding:"required"`
	Type  string `json:"type" form:"type"` // "school" or "student"
}

// OTP DTOs
type SendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	Type  string `json:"type"` // "school" or "student"
}

type VerifyOTPRequest struct {
	Email      string `json:"email" binding:"required,email"`
	OTP        string `json:"otp" binding:"required"`
	Type       string `json:"type"` // "school" or "student"
	RememberMe bool   `json:"rememberMe"`
}

// Auth Responses with success boolean envelope
type SchoolAuthResponse struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	School    SchoolResponse `json:"school"`
	Token     string         `json:"token,omitempty"`
	CSRFToken string         `json:"csrfToken,omitempty"`
}

type StudentAuthResponse struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	Student   StudentResponse `json:"student"`
	Token     string          `json:"token,omitempty"`
	CSRFToken string          `json:"csrfToken,omitempty"`
}

type CSRFResponse struct {
	Success   bool   `json:"success"`
	CSRFToken string `json:"csrfToken"`
}

type APIError struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type LogoutRequest struct {
	Email string `json:"email" form:"email"`
}


// User DTOs
type UserRegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"omitempty,oneof=ADMIN USER admin user"`
}

type UserLoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

type UserResponse struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	TwoFactorEnable bool       `json:"twoFactorEnable"`
	IsActivated     bool       `json:"isActivated"`
	IsVerified      bool       `json:"isVerified"`
	CSRFID          string     `json:"csrfID,omitempty"`
	SessionID       string     `json:"sessionId,omitempty"`
	IPAddress       string     `json:"ipAddress,omitempty"`
	RememberMe      bool       `json:"rememberMe"`
	SessionExpiry   *time.Time `json:"sessionExpiry,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type UserAuthResponse struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	User      UserResponse `json:"user"`
	Token     string       `json:"token,omitempty"`
	CSRFToken string       `json:"csrfToken,omitempty"`
}


