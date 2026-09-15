package dto

import "time"

// School DTOs
type SchoolRegisterRequest struct {
	SchoolName             string `json:"schoolName" binding:"required,min=2"`
	Board                  string `json:"board" binding:"required"`
	State                  string `json:"state" binding:"required"`
	District               string `json:"district" binding:"required"`
	City                   string `json:"city" binding:"required"`
	Address                string `json:"address" binding:"required"`
	Pincode                string `json:"pincode"`
	Email                  string `json:"email" binding:"required,email"`
	Phone                  string `json:"phone" binding:"required"`
	SchoolMobile           string `json:"schoolMobile"`
	Password               string `json:"password" binding:"required,min=6"`
	PrincipalName          string `json:"principalName" binding:"required"`
	CoordinatorName        string `json:"coordinatorName" binding:"required"`
	CoordinatorDesignation string `json:"coordinatorDesignation" binding:"required"`
	CoordinatorMobile      string `json:"coordinatorMobile" binding:"required"`
	CoordinatorEmail       string `json:"coordinatorEmail" binding:"required,email"`
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
	ClassCategory   string          `json:"classCategory,omitempty"`
	Category        string          `json:"category,omitempty"`
	Gender          string          `json:"gender,omitempty"`
	Status          string          `json:"status,omitempty"`
	AcademicYear    int             `json:"academicYear,omitempty"`
	Section         string          `json:"section,omitempty"`
	RollNo          string          `json:"rollNo,omitempty"`
	ParentName      string          `json:"parentName,omitempty"`
	ParentPhone     string          `json:"parentPhone,omitempty"`
	SchoolID        string          `json:"schoolId,omitempty"`
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

// RegisterPendingResponse is returned by RegisterSchool: registration
// succeeded and an activation email is on its way, but the account isn't
// activated yet, so there's no session/school profile to hand back — just
// the email, which the Activation Pending page needs to show "check your
// inbox" and to power its resend-email button.
type RegisterPendingResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Email   string `json:"email"`
}

// LoginPendingResponse is returned by LoginSchool once the password check
// passes: credentials are valid, but no session/cookie is issued yet — the
// caller still has to complete code verification (email OTP or MS Authenticator).
type LoginPendingResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Email   string `json:"email"`
}

// MfaStatusResponse tells the Code Verification screen whether Microsoft
// Authenticator has already been configured for this account.
type MfaStatusResponse struct {
	Success       bool   `json:"success"`
	MsAuthEnabled bool   `json:"msAuthEnabled"`
	Email         string `json:"email"`
}

// ResendActivationRequest is used by the frontend "Resend Activation Email" button.
type ResendActivationRequest struct {
	Email string `json:"email" binding:"required,email"`
	Type  string `json:"type"` // "school" or "student"
}

// MfaVerifySetupRequest confirms the TOTP code shown by the authenticator app
// during initial MFA setup (distinct from login-time code verification).
type MfaVerifySetupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Code     string `json:"code" binding:"required"`
	Type     string `json:"type"`
	ActToken string `json:"actToken" binding:"required"`
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


