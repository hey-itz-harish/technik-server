package dto

import "time"

type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AdminVerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required"`
}

type AdminUserDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Mobile    string    `json:"mobile,omitempty"`
	Role      string    `json:"role"`
	Zone      string    `json:"zone,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateAdminUserRequest struct {
	Name     string `json:"name" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Mobile   string `json:"mobile"`
	Zone     string `json:"zone"`
}

type UpdateNominationStatusRequest struct {
	Status      string `json:"status" binding:"required"`
	AdminStatus string `json:"adminStatus"`
}

type AdminStatsResponse struct {
	Success                bool  `json:"success"`
	TotalSchools           int   `json:"totalSchools"`
	TotalOlympiadStudents  int   `json:"totalOlympiadStudents"`
	TotalPrideNominations  int   `json:"totalPrideNominations"`
	PendingPrideNominations int   `json:"pendingPrideNominations"`
}

type AdminUsersResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Total   int            `json:"total"`
	Users   []AdminUserDTO `json:"users"`
}
