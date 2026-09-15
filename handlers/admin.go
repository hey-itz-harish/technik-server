package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"
	"technik-server/mail"
	"technik-server/middleware"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	cfg         *config.Config
	zohoService *mail.ZohoMailService
}

func NewAdminHandler(cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		cfg:         cfg,
		zohoService: mail.NewZohoMailService(cfg),
	}
}

// LoginAdmin handles authentication for Technik Portal Conductors & Super Admins
func (h *AdminHandler) LoginAdmin(c *gin.Context) {
	_ = database.EnsureConnected()
	var req dto.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	user, err := database.Client.User.FindUnique(
		db.User.Email.Equals(req.Email),
	).Exec(ctx)

	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Invalid admin email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Invalid admin email or password"})
		return
	}

	otpCode := fmt.Sprintf("%06d", rand.Intn(900000)+100000)
	otpExpiry := time.Now().Add(15 * time.Minute)

	_, _ = database.Client.User.FindUnique(
		db.User.ID.Equals(user.ID),
	).Update(
		db.User.Otp.Set(otpCode),
		db.User.OtpExpiry.Set(otpExpiry),
	).Exec(ctx)

	_ = h.zohoService.SendOTPEmail(user.Email, user.Name, otpCode)

	c.JSON(http.StatusOK, dto.LoginPendingResponse{
		Success: true,
		Message: "Admin credentials verified. OTP sent to email.",
		Email:   user.Email,
	})
}

// VerifyAdminOTP verifies OTP and issues admin JWT token
func (h *AdminHandler) VerifyAdminOTP(c *gin.Context) {
	_ = database.EnsureConnected()
	var req dto.AdminVerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	user, err := database.Client.User.FindUnique(
		db.User.Email.Equals(req.Email),
	).Exec(ctx)

	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Admin user not found"})
		return
	}

	savedOtp, _ := user.Otp()
	if savedOtp == "" || savedOtp != strings.TrimSpace(req.OTP) {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid OTP code"})
		return
	}

	jwtToken, err := middleware.GenerateJWT(user.ID, user.Email, "admin", h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to generate admin token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Admin verification successful",
		"token":   jwtToken,
		"user": dto.AdminUserDTO{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			Role:      string(user.Role),
			Status:    "Active & Verified",
			CreatedAt: user.CreatedAt,
		},
	})
}

// GetAdminStats calculates overview metrics for the Admin Dashboard
func (h *AdminHandler) GetAdminStats(c *gin.Context) {
	_ = database.EnsureConnected()
	ctx := context.Background()

	schools, _ := database.Client.SchoolDetails.FindMany().Exec(ctx)
	nominations, _ := database.Client.TechnikPrideNomination.FindMany().Exec(ctx)
	olympiadStudents, _ := database.Client.OlympiadStudent.FindMany().Exec(ctx)

	pendingCount := 0
	for _, nom := range nominations {
		st, _ := nom.NominationStatus()
		if st == "" || st == "Submitted & Under Review" || st == "Pending Review" {
			pendingCount++
		}
	}

	c.JSON(http.StatusOK, dto.AdminStatsResponse{
		Success:                 true,
		TotalSchools:            len(schools),
		TotalOlympiadStudents:   len(olympiadStudents),
		TotalPrideNominations:   len(nominations),
		PendingPrideNominations: pendingCount,
	})
}

// GetAdminPrideNominations lists all nominations across all schools
func (h *AdminHandler) GetAdminPrideNominations(c *gin.Context) {
	_ = database.EnsureConnected()
	ctx := context.Background()

	list, err := database.Client.TechnikPrideNomination.FindMany().With(
		db.TechnikPrideNomination.School.Fetch(),
	).OrderBy(
		db.TechnikPrideNomination.CreatedAt.Order(db.SortOrderDesc),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch nominations: " + err.Error()})
		return
	}

	var results []gin.H
	for _, nom := range list {
		schName := "Technik Partner School"
		schCity := "Main City"
		if sch, ok := nom.School(); ok && sch != nil {
			schName = sch.SchoolName
			c, _ := sch.City()
			if c != "" {
				schCity = c
			}
		}

		nomStat, _ := nom.NominationStatus()
		if nomStat == "" {
			nomStat = "Pending Review"
		}

		clsCat, _ := nom.ClassCategory()
		if clsCat == "" {
			if isJuniorLevel(nom.Class, "") {
				clsCat = "Junior Level"
			} else {
				clsCat = "Senior Level"
			}
		}

		doc, _ := nom.SupportingDocument()

		results = append(results, gin.H{
			"id":                  nom.ID,
			"studentName":         nom.StudentName,
			"schoolName":          schName,
			"schoolCity":          schCity,
			"grade":               nom.Class,
			"level":               clsCat,
			"category":            nom.AchievementCategory,
			"achievementTitle":    nom.AchievementTitle,
			"submissionDate":      nom.CreatedAt.Format("02 Jan 2006"),
			"proofFile":           doc,
			"status":              nomStat,
			"gender":              nom.Gender,
			"briefDescription":    nom.BriefDescription,
			"createdAt":           nom.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Pride nominations fetched successfully",
		"total":       len(results),
		"nominations": results,
	})
}

// UpdatePrideNominationStatus updates nomination status (Approved, Award Issued, Rejected)
func (h *AdminHandler) UpdatePrideNominationStatus(c *gin.Context) {
	_ = database.EnsureConnected()
	id := c.Param("id")
	var req dto.UpdateNominationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	updated, err := database.Client.TechnikPrideNomination.FindUnique(
		db.TechnikPrideNomination.ID.Equals(id),
	).Update(
		db.TechnikPrideNomination.NominationStatus.Set(req.Status),
		db.TechnikPrideNomination.AdminStatus.Set(req.AdminStatus),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update nomination status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Nomination status updated successfully",
		"id":      updated.ID,
		"status":  req.Status,
	})
}

// GetAdminOlympiadRegistrations fetches all olympiad registered students across all schools
func (h *AdminHandler) GetAdminOlympiadRegistrations(c *gin.Context) {
	_ = database.EnsureConnected()
	ctx := context.Background()

	registrations, err := database.Client.OlympiadRegistration.FindMany().With(
		db.OlympiadRegistration.School.Fetch(),
		db.OlympiadRegistration.Students.Fetch(),
	).OrderBy(
		db.OlympiadRegistration.CreatedAt.Order(db.SortOrderDesc),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch registrations: " + err.Error()})
		return
	}

	var results []gin.H
	for _, reg := range registrations {
		schName := "Technik Partner School"
		if sch, ok := reg.School(); ok && sch != nil {
			schName = sch.SchoolName
		}

		for _, st := range reg.Students() {
			results = append(results, gin.H{
				"rollNo":             st.ID,
				"studentName":        st.StudentName,
				"schoolName":         schName,
				"grade":              st.Class,
				"track":              st.OlympiadTrack,
				"examCenter":         "Official Technik Olympiad Center",
				"registeredDate":     st.CreatedAt.Format("02 Jan 2006"),
				"paymentStatus":      "Paid",
				"verificationStatus": "Hall Ticket Issued",
				"gender":             st.Gender,
				"createdAt":          st.CreatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Olympiad registrations fetched successfully",
		"total":         len(results),
		"registrations": results,
	})
}

// GetAdminUsers fetches all registered Admin / Conducting Professional users
func (h *AdminHandler) GetAdminUsers(c *gin.Context) {
	_ = database.EnsureConnected()
	ctx := context.Background()

	users, err := database.Client.User.FindMany().OrderBy(
		db.User.CreatedAt.Order(db.SortOrderDesc),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch admin users: " + err.Error()})
		return
	}

	var userDTOs []dto.AdminUserDTO
	for _, u := range users {
		userDTOs = append(userDTOs, dto.AdminUserDTO{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			Role:      string(u.Role),
			Status:    "Active & Verified",
			CreatedAt: u.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, dto.AdminUsersResponse{
		Success: true,
		Message: "Admin users fetched successfully",
		Total:   len(userDTOs),
		Users:   userDTOs,
	})
}

// CreateAdminUser registers a new Admin / Conducting Professional with unique email validation
func (h *AdminHandler) CreateAdminUser(c *gin.Context) {
	_ = database.EnsureConnected()
	var req dto.CreateAdminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()

	// Enforce Unique Email Check
	existing, err := database.Client.User.FindUnique(
		db.User.Email.Equals(strings.TrimSpace(req.Email)),
	).Exec(ctx)

	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, dto.APIError{
			Success: false,
			Error:   "Username / Email already registered! An admin user with this email address already exists.",
		})
		return
	}

	password := req.Password
	if password == "" {
		password = "TechnikPass#2026"
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to encrypt password"})
		return
	}

	newUser, err := database.Client.User.CreateOne(
		db.User.Name.Set(strings.TrimSpace(req.Name)),
		db.User.Email.Set(strings.TrimSpace(req.Email)),
		db.User.Password.Set(string(hashedPass)),
		db.User.Role.Set(db.RoleAdmin),
		db.User.IsActivated.Set(true),
		db.User.IsVerified.Set(true),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create admin user: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Admin user created successfully",
		"user": dto.AdminUserDTO{
			ID:        newUser.ID,
			Name:      newUser.Name,
			Email:     newUser.Email,
			Role:      string(newUser.Role),
			Status:    "Active & Verified",
			CreatedAt: newUser.CreatedAt,
		},
	})
}
