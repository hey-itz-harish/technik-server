package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
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
	"github.com/google/uuid"
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

	// Verify portal user is not soft-deleted
	sqlDB := database.GetSQLDB()
	if sqlDB != nil {
		var isDeleted bool
		_ = sqlDB.QueryRow(`SELECT COALESCE(deleted, false) FROM "User" WHERE id = $1;`, user.ID).Scan(&isDeleted)
		if isDeleted {
			c.JSON(http.StatusForbidden, dto.APIError{
				Success: false,
				Error:   "This portal user account has been deactivated (soft-deleted). Please contact Super Admin.",
			})
			return
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Invalid admin email or password"})
		return
	}

	otpBytes := make([]byte, 3)
	_, _ = rand.Read(otpBytes)
	otpNum := (int(otpBytes[0])<<16 | int(otpBytes[1])<<8 | int(otpBytes[2])) % 1000000
	otpCode := fmt.Sprintf("%06d", otpNum)
	otpExpiry := time.Now().Add(10 * time.Minute)

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

	// Verify portal user is not soft-deleted
	sqlDB := database.GetSQLDB()
	if sqlDB != nil {
		var isDeleted bool
		_ = sqlDB.QueryRow(`SELECT COALESCE(deleted, false) FROM "User" WHERE id = $1;`, user.ID).Scan(&isDeleted)
		if isDeleted {
			c.JSON(http.StatusForbidden, dto.APIError{
				Success: false,
				Error:   "This portal user account has been deactivated (soft-deleted). Please contact Super Admin.",
			})
			return
		}
	}

	savedOtp, _ := user.Otp()
	exp, hasExp := user.OtpExpiry()
	now := time.Now()

	if savedOtp == "" || savedOtp != strings.TrimSpace(req.OTP) {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid OTP code"})
		return
	}

	if !hasExp || exp.Before(now) {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Admin OTP code has expired. Please log in again."})
		return
	}

	// Invalidate OTP immediately to prevent reuse / replay attacks
	_, _ = database.Client.User.FindUnique(
		db.User.ID.Equals(user.ID),
	).Update(
		db.User.Otp.Set(""),
	).Exec(ctx)

	jwtToken, err := middleware.GenerateJWT(user.ID, user.Email, "admin", h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to generate admin token"})
		return
	}

	csrfToken := middleware.GenerateCSRFToken(h.cfg.CSRFSecret)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.JWTCookieName, jwtToken, 86400, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
	c.SetCookie(middleware.CSRFCookieName, csrfToken, 86400, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, false)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Admin verification successful",
		"token":     jwtToken,
		"csrfToken": csrfToken,
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
			clsCat = "Technik Pride Award"
		}

		doc, _ := nom.SupportingDocument()

		results = append(results, gin.H{
			"id":               nom.ID,
			"studentName":      nom.StudentName,
			"schoolName":       schName,
			"schoolCity":       schCity,
			"grade":            nom.Class,
			"level":            clsCat,
			"category":         nom.AchievementCategory,
			"achievementTitle": nom.AchievementTitle,
			"submissionDate":   nom.CreatedAt.Format("02 Jan 2006"),
			"proofFile":        doc,
			"status":           nomStat,
			"gender":           nom.Gender,
			"briefDescription": nom.BriefDescription,
			"createdAt":        nom.CreatedAt,
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

	// Also sync status into StudentDetails if matching student exists
	studentName := updated.StudentName
	var studentWhere []db.StudentDetailsWhereParam
	studentWhere = append(studentWhere, db.StudentDetails.StudentName.Equals(studentName))
	if schID, ok := updated.SchoolID(); ok && schID != "" {
		studentWhere = append(studentWhere, db.StudentDetails.SchoolID.Equals(schID))
	}
	matchedStudents, _ := database.Client.StudentDetails.FindMany(studentWhere...).Exec(ctx)
	for _, st := range matchedStudents {
		_, _ = database.Client.StudentDetails.FindUnique(
			db.StudentDetails.ID.Equals(st.ID),
		).Update(
			db.StudentDetails.Status.Set(req.Status),
		).Exec(ctx)
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

// GetAdminUsers fetches registered Admin / Conducting Professional users
func (h *AdminHandler) GetAdminUsers(c *gin.Context) {
	_ = database.EnsureConnected()

	includeDeleted := strings.EqualFold(c.Query("includeDeleted"), "true")

	sqlDB := database.GetSQLDB()
	if sqlDB != nil {
		query := `
			SELECT id, name, email, COALESCE(role::text, 'ADMIN'), COALESCE(role_id, ''), COALESCE(role_name, ''),
			       COALESCE(mobile, ''), COALESCE(zone, ''), COALESCE(deleted, false), deleted_at, created_at
			FROM "User"`
		if !includeDeleted {
			query += ` WHERE COALESCE(deleted, false) = false`
		}
		query += ` ORDER BY created_at DESC;`

		rows, err := sqlDB.Query(query)
		if err == nil {
			defer rows.Close()
			var userDTOs []dto.AdminUserDTO
			for rows.Next() {
				var u dto.AdminUserDTO
				var roleID, roleName, mobile, zone string
				var deletedAt sql.NullTime

				if err := rows.Scan(
					&u.ID, &u.Name, &u.Email, &u.Role,
					&roleID, &roleName, &mobile, &zone,
					&u.Deleted, &deletedAt, &u.CreatedAt,
				); err == nil {
					u.RoleID = roleID
					u.Mobile = mobile
					u.Zone = zone
					if roleName != "" {
						u.Role = roleName
					}
					if deletedAt.Valid {
						u.DeletedAt = &deletedAt.Time
					}
					if u.Deleted {
						u.Status = "Deactivated (Deleted)"
					} else {
						u.Status = "Active & Verified"
					}
					userDTOs = append(userDTOs, u)
				}
			}

			c.JSON(http.StatusOK, dto.AdminUsersResponse{
				Success: true,
				Message: "Admin users fetched successfully",
				Total:   len(userDTOs),
				Users:   userDTOs,
			})
			return
		}
	}

	// Fallback to Prisma if SQL query is not executed
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
			Deleted:   false,
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

	roleName := req.Role
	if roleName == "" {
		roleName = "SUPER_ADMIN"
	}

	// Update roles/mobile/zone and ensure deleted = FALSE in SQL
	sqlDB := database.GetSQLDB()
	if sqlDB != nil {
		if req.RoleID != "" && req.Role == "" {
			var rName string
			_ = sqlDB.QueryRow(`SELECT name FROM roles WHERE id = $1 AND portal_only = TRUE;`, req.RoleID).Scan(&rName)
			if rName != "" {
				roleName = rName
			}
		}

		_, _ = sqlDB.Exec(`
			UPDATE "User"
			SET deleted = FALSE,
			    deleted_at = NULL,
			    role_id = $1,
			    role_name = $2,
			    mobile = $3,
			    zone = $4
			WHERE id = $5;
		`, req.RoleID, roleName, strings.TrimSpace(req.Mobile), strings.TrimSpace(req.Zone), newUser.ID)
	}

	// Send Welcome & Passcode Email via Resend/Zoho using noreply@technikolympiad.com
	_ = h.zohoService.SendAdminWelcomeEmail(newUser.Email, newUser.Name, roleName, req.Zone, password)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Admin user registered and official passcode email sent successfully",
		"user": dto.AdminUserDTO{
			ID:        newUser.ID,
			Name:      newUser.Name,
			Email:     newUser.Email,
			Mobile:    req.Mobile,
			Role:      roleName,
			RoleID:    req.RoleID,
			Zone:      req.Zone,
			Status:    "Active & Verified",
			Deleted:   false,
			CreatedAt: newUser.CreatedAt,
		},
	})
}

// SoftDeleteAdminUser performs soft deletion of a Technik portal user
func (h *AdminHandler) SoftDeleteAdminUser(c *gin.Context) {
	_ = database.EnsureConnected()
	targetID := strings.TrimSpace(c.Param("id"))
	if targetID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "User ID is required"})
		return
	}

	currentUserID, exists := c.Get("user_id")
	if exists && currentUserID == targetID {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "You cannot deactivate your own portal account"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	var existsInDB bool
	err := sqlDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM "User" WHERE id = $1);`, targetID).Scan(&existsInDB)
	if err != nil || !existsInDB {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Portal user not found"})
		return
	}

	_, err = sqlDB.Exec(`
		UPDATE "User"
		SET deleted = TRUE,
		    deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`, targetID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to deactivate portal user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Portal user deactivated (soft-deleted) successfully",
		"id":      targetID,
		"deleted": true,
	})
}

// RestoreAdminUser restores a previously soft-deleted Technik portal user
func (h *AdminHandler) RestoreAdminUser(c *gin.Context) {
	_ = database.EnsureConnected()
	targetID := strings.TrimSpace(c.Param("id"))
	if targetID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "User ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	var existsInDB bool
	err := sqlDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM "User" WHERE id = $1);`, targetID).Scan(&existsInDB)
	if err != nil || !existsInDB {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Portal user not found"})
		return
	}

	_, err = sqlDB.Exec(`
		UPDATE "User"
		SET deleted = FALSE,
		    deleted_at = NULL
		WHERE id = $1;
	`, targetID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to restore portal user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Portal user account restored successfully",
		"id":      targetID,
		"deleted": false,
	})
}

// AssignUserRole assigns a portal role from the roles table to a Technik portal user
func (h *AdminHandler) AssignUserRole(c *gin.Context) {
	_ = database.EnsureConnected()
	targetID := strings.TrimSpace(c.Param("id"))
	if targetID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "User ID is required"})
		return
	}

	var req dto.AssignUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	var existsInDB bool
	err := sqlDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM "User" WHERE id = $1);`, targetID).Scan(&existsInDB)
	if err != nil || !existsInDB {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Portal user not found"})
		return
	}

	roleName := req.RoleName
	if req.RoleID != "" {
		var dbRoleName string
		var portalOnly bool
		err := sqlDB.QueryRow(`SELECT name, portal_only FROM roles WHERE id = $1 AND COALESCE(deleted, false) = false;`, req.RoleID).Scan(&dbRoleName, &portalOnly)
		if err != nil || !portalOnly {
			c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid or inactive portal role. Roles are applicable only to Technik portal users."})
			return
		}
		roleName = dbRoleName
	}

	_, err = sqlDB.Exec(`
		UPDATE "User"
		SET role_id = $1,
		    role_name = $2
		WHERE id = $3;
	`, req.RoleID, roleName, targetID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to assign role to portal user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Portal user role updated successfully",
		"userId":   targetID,
		"roleId":   req.RoleID,
		"roleName": roleName,
	})
}

// GetPortalRoles lists all Technik portal roles (portal_only = TRUE, non-deleted)
func (h *AdminHandler) GetPortalRoles(c *gin.Context) {
	_ = database.EnsureConnected()
	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	rows, err := sqlDB.Query(`
		SELECT id, name, code, COALESCE(description, ''), portal_only,
		       COALESCE(permissions::text, '[]'), is_active, COALESCE(deleted, false), created_at, updated_at
		FROM roles
		WHERE portal_only = TRUE AND COALESCE(deleted, false) = false
		ORDER BY name ASC;
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch portal roles: " + err.Error()})
		return
	}
	defer rows.Close()

	var roles []dto.PortalRoleDTO
	for rows.Next() {
		var r dto.PortalRoleDTO
		var permsJSON string
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Code, &r.Description, &r.PortalOnly,
			&permsJSON, &r.IsActive, &r.Deleted, &r.CreatedAt, &r.UpdatedAt,
		); err == nil {
			var perms []string
			if err := json.Unmarshal([]byte(permsJSON), &perms); err != nil {
				perms = []string{}
			}
			r.Permissions = perms
			roles = append(roles, r)
		}
	}

	c.JSON(http.StatusOK, dto.PortalRolesResponse{
		Success: true,
		Message: "Technik portal roles fetched successfully",
		Total:   len(roles),
		Roles:   roles,
	})
}

// CreatePortalRole creates a new role strictly for Technik Portal users (portal_only = TRUE)
func (h *AdminHandler) CreatePortalRole(c *gin.Context) {
	_ = database.EnsureConnected()
	var req dto.CreatePortalRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	name := strings.TrimSpace(req.Name)

	var exists bool
	_ = sqlDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE code = $1 AND COALESCE(deleted, false) = false);`, code).Scan(&exists)
	if exists {
		c.JSON(http.StatusConflict, dto.APIError{Success: false, Error: "A portal role with code '" + code + "' already exists"})
		return
	}

	if req.Permissions == nil {
		req.Permissions = []string{}
	}
	permsJSON, err := json.Marshal(req.Permissions)
	if err != nil {
		permsJSON = []byte("[]")
	}

	newID := "ROLE-" + strings.ToUpper(uuid.New().String()[:8])
	now := time.Now()

	_, err = sqlDB.Exec(`
		INSERT INTO roles (id, name, code, description, portal_only, permissions, is_active, deleted, created_at, updated_at)
		VALUES ($1, $2, $3, $4, TRUE, $5::jsonb, TRUE, FALSE, $6, $6);
	`, newID, name, code, req.Description, string(permsJSON), now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create portal role: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Technik portal role created successfully",
		"role": dto.PortalRoleDTO{
			ID:          newID,
			Name:        name,
			Code:        code,
			Description: req.Description,
			PortalOnly:  true,
			Permissions: req.Permissions,
			IsActive:    true,
			Deleted:     false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	})
}

// UpdatePortalRole updates an existing Technik portal role
func (h *AdminHandler) UpdatePortalRole(c *gin.Context) {
	_ = database.EnsureConnected()
	roleID := strings.TrimSpace(c.Param("id"))
	if roleID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Role ID is required"})
		return
	}

	var req dto.UpdatePortalRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	var exists bool
	err := sqlDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1 AND portal_only = TRUE AND COALESCE(deleted, false) = false);`, roleID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Portal role not found or not editable"})
		return
	}

	permsJSON, _ := json.Marshal(req.Permissions)

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	_, err = sqlDB.Exec(`
		UPDATE roles
		SET name = COALESCE(NULLIF($1, ''), name),
		    description = $2,
		    permissions = COALESCE($3::jsonb, permissions),
		    is_active = $4,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $5 AND portal_only = TRUE;
	`, req.Name, req.Description, string(permsJSON), isActive, roleID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update portal role: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Portal role updated successfully",
		"id":      roleID,
	})
}

// DeletePortalRole soft-deletes a Technik portal role
func (h *AdminHandler) DeletePortalRole(c *gin.Context) {
	_ = database.EnsureConnected()
	roleID := strings.TrimSpace(c.Param("id"))
	if roleID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Role ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	var roleCode string
	err := sqlDB.QueryRow(`SELECT code FROM roles WHERE id = $1 AND portal_only = TRUE;`, roleID).Scan(&roleCode)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Portal role not found"})
		return
	}
	if roleCode == "SUPER_ADMIN" {
		c.JSON(http.StatusForbidden, dto.APIError{Success: false, Error: "The Super Admin system role cannot be deleted"})
		return
	}

	_, err = sqlDB.Exec(`
		UPDATE roles
		SET deleted = TRUE,
		    deleted_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND portal_only = TRUE;
	`, roleID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to delete portal role: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Portal role soft-deleted successfully",
		"id":      roleID,
		"deleted": true,
	})
}
