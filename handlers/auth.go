package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"
	"technik-server/logger"
	"technik-server/mail"
	"technik-server/middleware"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	cfg         *config.Config
	zohoService *mail.ZohoMailService
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		zohoService: mail.NewZohoMailService(cfg),
	}
}

// GenerateSchoolCode generates a unique school code string like SCH-2026-TXI
func GenerateSchoolCode() string {
	year := time.Now().Year()
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 3)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("SCH-%d-%03d", year, time.Now().UnixNano()%1000)
	}

	suffix := make([]byte, 3)
	for i, v := range b {
		suffix[i] = charset[int(v)%len(charset)]
	}

	return fmt.Sprintf("SCH-%d-%s", year, string(suffix))
}

// GenerateNumericOTP generates a 6-digit numeric OTP code
func GenerateNumericOTP() string {
	b := make([]byte, 3)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	num := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
	return fmt.Sprintf("%06d", num)
}

func parseSessionExpiry(t time.Time, ok bool) *time.Time {
	if !ok || t.IsZero() {
		return nil
	}
	return &t
}

// GetCSRFToken returns a valid CSRF token in the response payload
func (h *AuthHandler) GetCSRFToken(c *gin.Context) {
	token, _ := c.Cookie(middleware.CSRFCookieName)
	if token == "" {
		token = middleware.GenerateCSRFToken(h.cfg.CSRFSecret)
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(
			middleware.CSRFCookieName,
			token,
			86400,
			"/",
			h.cfg.CookieDomain,
			h.cfg.CookieSecure,
			false,
		)
	}

	c.JSON(http.StatusOK, dto.CSRFResponse{
		Success:   true,
		CSRFToken: token,
	})
}

// ActivateAccount handles account activation using token sent via email link
func (h *AuthHandler) ActivateAccount(c *gin.Context) {
	token := c.Query("token")
	entityType := c.Query("type")

	if token == "" {
		var req dto.ActivateAccountRequest
		if err := c.ShouldBind(&req); err == nil && req.Token != "" {
			token = req.Token
			if req.Type != "" {
				entityType = req.Type
			}
		}
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Activation token is required"})
		return
	}

	ctx := context.Background()
	now := time.Now()

	if entityType == "school" || entityType == "" {
		school, err := database.Client.SchoolDetails.FindFirst(
			db.SchoolDetails.ActivationToken.Equals(token),
		).Exec(ctx)

		if err == nil && school != nil {
			exp, ok := school.ActivationExpiry()
			if ok && exp.Before(now) {
				c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Activation token has expired. Please request a new link."})
				return
			}

			_, err = database.Client.SchoolDetails.FindUnique(
				db.SchoolDetails.ID.Equals(school.ID),
			).Update(
				db.SchoolDetails.IsActivated.Set(true),
				db.SchoolDetails.ActivationToken.Set(""),
			).Exec(ctx)

			if err != nil {
				c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to activate school account"})
				return
			}

			// Mint a short-lived token authorizing the immediate follow-up
			// MFA setup step, so /api/auth/mfa/setup isn't reachable by
			// email alone.
			mfaSetupToken, _ := middleware.GeneratePurposeToken(school.Email, "school", "mfa-setup", h.cfg.JWTSecret, 30*time.Minute)

			c.JSON(http.StatusOK, gin.H{
				"success":       true,
				"isActivated":   true,
				"message":       "School account activated successfully! You can now set up Microsoft Authenticator or continue to login.",
				"email":         school.Email,
				"schoolName":    school.SchoolName,
				"mfaSetupToken": mfaSetupToken,
			})
			return
		}
	}

	if entityType == "student" || entityType == "" {
		student, err := database.Client.StudentDetails.FindFirst(
			db.StudentDetails.ActivationToken.Equals(token),
		).Exec(ctx)

		if err == nil && student != nil {
			exp, ok := student.ActivationExpiry()
			if ok && exp.Before(now) {
				c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Activation token has expired. Please request a new link."})
				return
			}

			_, err = database.Client.StudentDetails.FindUnique(
				db.StudentDetails.ID.Equals(student.ID),
			).Update(
				db.StudentDetails.IsActivated.Set(true),
				db.StudentDetails.ActivationToken.Set(""),
			).Exec(ctx)

			if err != nil {
				c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to activate student account"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success":     true,
				"isActivated": true,
				"message":     "Student account activated successfully! You can now login.",
			})
			return
		}
	}

	c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid activation token"})
}

// GetActivationStatus returns the current activation status (isActivated: true/false) for a school or student.
func (h *AuthHandler) GetActivationStatus(c *gin.Context) {
	email := c.Query("email")
	token := c.Query("token")
	entityType := c.Query("type")

	if email == "" && token == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "email or token is required"})
		return
	}

	ctx := context.Background()

	if entityType == "school" || entityType == "" {
		var school *db.SchoolDetailsModel
		var err error

		if email != "" {
			school, err = database.Client.SchoolDetails.FindUnique(
				db.SchoolDetails.Email.Equals(email),
			).Exec(ctx)
		} else if token != "" {
			school, err = database.Client.SchoolDetails.FindFirst(
				db.SchoolDetails.ActivationToken.Equals(token),
			).Exec(ctx)
		}

		if err == nil && school != nil {
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"isActivated":     school.IsActivated,
				"email":           school.Email,
				"schoolName":      school.SchoolName,
				"twoFactorEnable": school.TwoFactorEnable,
			})
			return
		}
	}

	if entityType == "student" || entityType == "" {
		var student *db.StudentDetailsModel
		var err error

		if email != "" {
			student, err = database.Client.StudentDetails.FindUnique(
				db.StudentDetails.Email.Equals(email),
			).Exec(ctx)
		} else if token != "" {
			student, err = database.Client.StudentDetails.FindFirst(
				db.StudentDetails.ActivationToken.Equals(token),
			).Exec(ctx)
		}

		if err == nil && student != nil {
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"isActivated":     student.IsActivated,
				"email":           student.Email,
				"twoFactorEnable": student.TwoFactorEnable,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Account not found"})
}

// buildResendLink signs a long-lived (7 day) purpose token for the given
// school email and returns the backend URL that resends the activation
// email when opened. Returns an empty string if signing fails.
func (h *AuthHandler) buildResendLink(email string) string {
	resendToken, err := middleware.GeneratePurposeToken(email, "school", "resend-activation", h.cfg.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s/api/auth/resend-activation?rtoken=%s&type=school", h.cfg.Port, resendToken)
}

// resendActivationInternal regenerates a fresh activation token/expiry for
// the given school email and re-sends the activation email (with a new
// resend link of its own). Returns an error describing why the resend
// couldn't be completed.
func (h *AuthHandler) resendActivationInternal(ctx context.Context, email string) error {
	school, err := database.Client.SchoolDetails.FindUnique(
		db.SchoolDetails.Email.Equals(email),
	).Exec(ctx)

	if err != nil || school == nil {
		return fmt.Errorf("no school account was found for this email")
	}

	if school.IsActivated {
		return fmt.Errorf("this account is already activated — please proceed to login")
	}

	activationToken := uuid.New().String()
	activationExpiry := time.Now().Add(24 * time.Hour)

	_, err = database.Client.SchoolDetails.FindUnique(
		db.SchoolDetails.ID.Equals(school.ID),
	).Update(
		db.SchoolDetails.ActivationToken.Set(activationToken),
		db.SchoolDetails.ActivationExpiry.Set(activationExpiry),
	).Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to regenerate the activation token")
	}

	activationLink := fmt.Sprintf("%s/activation-pending?token=%s&type=school", h.cfg.FrontendURL, activationToken)
	resendLink := h.buildResendLink(email)
	go func(toEmail, schoolName, actLink, resLink string) {
		_ = h.zohoService.SendActivationEmail(toEmail, schoolName, actLink, resLink)
	}(email, school.SchoolName, activationLink, resendLink)

	return nil
}

// resendResultHTML renders a small, self-contained confirmation page since
// this link is opened directly from the email client, not via the frontend SPA.
func resendResultHTML(success bool, message string) string {
	title := "Activation Email Resent"
	color := "#16a34a"
	if !success {
		title = "Couldn't Resend Activation Email"
		color = "#dc2626"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title></head>
<body style="font-family: Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 40px 20px;">
  <div style="max-width: 480px; margin: 0 auto; background: #ffffff; padding: 32px; border-radius: 16px; border: 1px solid #e2e8f0; box-shadow: 0 4px 20px rgba(0,0,0,0.05); text-align: center;">
    <h2 style="color: %s; margin: 0 0 12px 0;">%s</h2>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">%s</p>
  </div>
</body>
</html>`, title, color, title, message)
}

// ResendActivationByToken is hit directly from the "Resend Activation Link"
// button embedded in the activation email. It validates the long-lived
// resend token and reissues a fresh activation email.
func (h *AuthHandler) ResendActivationByToken(c *gin.Context) {
	rtoken := c.Query("rtoken")
	if rtoken == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(resendResultHTML(false, "This resend link is missing its token.")))
		return
	}

	claims, err := middleware.ValidatePurposeToken(rtoken, "resend-activation", h.cfg.JWTSecret)
	if err != nil {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(resendResultHTML(false, "This resend link is invalid or has expired. Please contact support.")))
		return
	}

	if err := h.resendActivationInternal(context.Background(), claims.Email); err != nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(resendResultHTML(false, err.Error())))
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(resendResultHTML(true, fmt.Sprintf("A fresh activation link has been sent to %s. Please check your inbox.", claims.Email))))
}

// ResendActivationByEmail powers the "Resend Activation Email" button on the
// frontend's activation-pending waiting page.
func (h *AuthHandler) ResendActivationByEmail(c *gin.Context) {
	var req dto.ResendActivationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	if req.Type != "" && req.Type != "school" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Resend is currently only supported for school accounts"})
		return
	}

	if err := h.resendActivationInternal(context.Background(), req.Email); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "A fresh activation link has been sent to your email.",
	})
}

// SendOTP generates and emails a 6-digit OTP code using Zoho Mail Service
func (h *AuthHandler) SendOTP(c *gin.Context) {
	var req dto.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	otpCode := GenerateNumericOTP()
	otpExpiry := time.Now().Add(10 * time.Minute)

	var recipientName string

	if req.Type == "school" {
		school, err := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.Email.Equals(req.Email),
		).Exec(ctx)

		if err != nil || school == nil {
			c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "School with this email not found"})
			return
		}

		if !school.IsActivated {
			c.JSON(http.StatusForbidden, dto.APIError{Success: false, Error: "Account is not activated. Please activate your account using the email link before proceeding with MFA/OTP."})
			return
		}

		recipientName = school.SchoolName

		_, err = database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.ID.Equals(school.ID),
		).Update(
			db.SchoolDetails.Otp.Set(otpCode),
			db.SchoolDetails.OtpExpiry.Set(otpExpiry),
		).Exec(ctx)

		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update OTP code"})
			return
		}
	} else {
		// Student OTP
		student, err := database.Client.StudentDetails.FindFirst(
			db.StudentDetails.Email.Equals(req.Email),
		).Exec(ctx)

		if err != nil || student == nil {
			c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student with this email not found"})
			return
		}

		if !student.IsActivated {
			c.JSON(http.StatusForbidden, dto.APIError{Success: false, Error: "Account is not activated. Please activate your account using the email link before proceeding with MFA/OTP."})
			return
		}

		recipientName = student.StudentName

		_, err = database.Client.StudentDetails.FindUnique(
			db.StudentDetails.ID.Equals(student.ID),
		).Update(
			db.StudentDetails.Otp.Set(otpCode),
			db.StudentDetails.OtpExpiry.Set(otpExpiry),
		).Exec(ctx)

		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update OTP code"})
			return
		}
	}

	// Send OTP email via Zoho Mail Service
	if err := h.zohoService.SendOTPEmail(req.Email, recipientName, otpCode); err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to send OTP email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP code sent successfully to email",
	})
}

// VerifyOTP validates the OTP code, updates isVerified: true, and issues session tokens/cookies
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req dto.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	now := time.Now()
	clientIP := c.ClientIP()
	mode := c.DefaultQuery("mode", "mail")

	if req.Type == "school" {
		school, err := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.Email.Equals(req.Email),
		).Exec(ctx)

		if err != nil || school == nil {
			c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "School not found"})
			return
		}

		if !school.IsActivated {
			c.JSON(http.StatusForbidden, dto.APIError{Success: false, Error: "Account is not activated. Please activate your account via email first."})
			return
		}

		if mode == "msauth" {
			if !school.TwoFactorEnable {
				c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Microsoft Authenticator is not set up for this account."})
				return
			}
			secret, hasSecret := school.MsAuthSecret()
			if !hasSecret || secret == "" || !validateTOTPCode(secret, req.OTP) {
				c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid authenticator code"})
				return
			}
		} else {
			savedOtp, _ := school.Otp()
			exp, ok := school.OtpExpiry()

			if savedOtp == "" || savedOtp != req.OTP {
				c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid OTP code"})
				return
			}

			if !ok || exp.Before(now) {
				c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "OTP code has expired. Please request a new OTP."})
				return
			}
		}

		jwtToken, err := middleware.GenerateJWT(school.ID, school.Email, "school", h.cfg.JWTSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to generate token"})
			return
		}

		sessionID := uuid.New().String()
		csrfToken := middleware.GenerateCSRFToken(h.cfg.CSRFSecret)
		sessionExpiry := now.Add(24 * time.Hour)

		// Verification Success: Update DB fields (isVerified: true, session parameters)
		updatedSchool, err := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.ID.Equals(school.ID),
		).Update(
			db.SchoolDetails.IsActivated.Set(true),
			db.SchoolDetails.IsVerified.Set(true),
			db.SchoolDetails.Otp.Set(""),
			db.SchoolDetails.RememberMe.Set(req.RememberMe),
			db.SchoolDetails.IPAddress.Set(clientIP),
			db.SchoolDetails.SessionID.Set(sessionID),
			db.SchoolDetails.CsrfID.Set(csrfToken),
			db.SchoolDetails.SessionExpiry.Set(sessionExpiry),
		).Exec(ctx)

		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update school verification status"})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(middleware.JWTCookieName, jwtToken, 86400, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
		c.SetCookie(middleware.CSRFCookieName, csrfToken, 86400, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, false)

		board, _ := updatedSchool.Board()
		state, _ := updatedSchool.State()
		district, _ := updatedSchool.District()
		city, _ := updatedSchool.City()
		address, _ := updatedSchool.Address()
		pincode, _ := updatedSchool.Pincode()
		phone, _ := updatedSchool.Phone()
		principal, _ := updatedSchool.PrincipalName()
		coordName, _ := updatedSchool.CoordinatorName()
		coordDesig, _ := updatedSchool.CoordinatorDesignation()
		coordMob, _ := updatedSchool.CoordinatorMobile()
		coordEmail, _ := updatedSchool.CoordinatorEmail()
		cID, _ := updatedSchool.CsrfID()
		sID, _ := updatedSchool.SessionID()
		ip, _ := updatedSchool.IPAddress()
		sExp := parseSessionExpiry(updatedSchool.SessionExpiry())

		// Record the session start now — LoginSchool no longer issues a
		// session by itself, so this is the actual point of login.
		_, _ = database.Client.SessionLog.CreateOne(
			db.SessionLog.UserEmail.Set(updatedSchool.Email),
			db.SessionLog.Action.Set("LOGIN"),
			db.SessionLog.IsActive.Set(true),
			db.SessionLog.IPAddress.Set(clientIP),
			db.SessionLog.UserAgent.Set(c.Request.UserAgent()),
			db.SessionLog.SessionID.Set(sessionID),
		).Exec(ctx)

		c.JSON(http.StatusOK, dto.SchoolAuthResponse{
			Success: true,
			Message: "Verification successful",
			School: dto.SchoolResponse{
				ID:                     updatedSchool.ID,
				SchoolCode:             updatedSchool.SchoolCode,
				SchoolName:             updatedSchool.SchoolName,
				Email:                  updatedSchool.Email,
				Board:                  board,
				State:                  state,
				District:               district,
				City:                   city,
				Address:                address,
				Pincode:                pincode,
				Phone:                  phone,
				PrincipalName:          principal,
				CoordinatorName:        coordName,
				CoordinatorDesignation: coordDesig,
				CoordinatorMobile:      coordMob,
				CoordinatorEmail:       coordEmail,
				TwoFactorEnable:        updatedSchool.TwoFactorEnable,
				IsActivated:            updatedSchool.IsActivated,
				IsVerified:             updatedSchool.IsVerified,
				CSRFID:                 cID,
				SessionID:              sID,
				IPAddress:              ip,
				RememberMe:             updatedSchool.RememberMe,
				SessionExpiry:          sExp,
				CreatedAt:              updatedSchool.CreatedAt,
				UpdatedAt:              updatedSchool.UpdatedAt,
			},
			Token:     jwtToken,
			CSRFToken: csrfToken,
		})
		return
	}

	// Student OTP Verification
	student, err := database.Client.StudentDetails.FindFirst(
		db.StudentDetails.Email.Equals(req.Email),
	).With(db.StudentDetails.School.Fetch()).Exec(ctx)

	if err != nil || student == nil {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student not found"})
		return
	}

	if !student.IsActivated {
		c.JSON(http.StatusForbidden, dto.APIError{Success: false, Error: "Account is not activated. Please activate your account via email first."})
		return
	}

	savedOtp, _ := student.Otp()
	exp, ok := student.OtpExpiry()

	if savedOtp == "" || savedOtp != req.OTP {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid OTP code"})
		return
	}

	if !ok || exp.Before(now) {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "OTP code has expired. Please request a new OTP."})
		return
	}

	email, _ := student.Email()
	jwtToken, err := middleware.GenerateJWT(student.ID, email, "student", h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to generate token"})
		return
	}

	sessionID := uuid.New().String()
	csrfToken := middleware.GenerateCSRFToken(h.cfg.CSRFSecret)
	sessionExpiry := now.Add(24 * time.Hour)

	updatedStudent, err := database.Client.StudentDetails.FindUnique(
		db.StudentDetails.ID.Equals(student.ID),
	).Update(
		db.StudentDetails.IsActivated.Set(true),
		db.StudentDetails.IsVerified.Set(true),
		db.StudentDetails.Otp.Set(""),
		db.StudentDetails.RememberMe.Set(req.RememberMe),
		db.StudentDetails.IPAddress.Set(clientIP),
		db.StudentDetails.SessionID.Set(sessionID),
		db.StudentDetails.CsrfID.Set(csrfToken),
		db.StudentDetails.SessionExpiry.Set(sessionExpiry),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update student verification status"})
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.JWTCookieName, jwtToken, 86400, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
	c.SetCookie(middleware.CSRFCookieName, csrfToken, 86400, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, false)

	phone, _ := updatedStudent.Phone()
	section, _ := updatedStudent.Section()
	rollNo, _ := updatedStudent.RollNo()
	parentName, _ := updatedStudent.ParentName()
	parentPhone, _ := updatedStudent.ParentPhone()
	cID, _ := updatedStudent.CsrfID()
	sID, _ := updatedStudent.SessionID()
	ip, _ := updatedStudent.IPAddress()
	sExp := parseSessionExpiry(updatedStudent.SessionExpiry())

	c.JSON(http.StatusOK, dto.StudentAuthResponse{
		Success: true,
		Message: "OTP verification successful",
		Student: dto.StudentResponse{
			ID:              updatedStudent.ID,
			StudentName:     updatedStudent.StudentName,
			Email:           email,
			Phone:           phone,
			Grade:           updatedStudent.Grade,
			Section:         section,
			RollNo:          rollNo,
			ParentName:      parentName,
			ParentPhone:     parentPhone,
			TwoFactorEnable: updatedStudent.TwoFactorEnable,
			IsActivated:     updatedStudent.IsActivated,
			IsVerified:      updatedStudent.IsVerified,
			CSRFID:          cID,
			SessionID:       sID,
			IPAddress:       ip,
			RememberMe:      updatedStudent.RememberMe,
			SessionExpiry:   sExp,
			CreatedAt:       updatedStudent.CreatedAt,
			UpdatedAt:       updatedStudent.UpdatedAt,
		},
		Token:     jwtToken,
		CSRFToken: csrfToken,
	})
}

// RegisterSchool registers a new school with isActivated: false and sends activation email link
func (h *AuthHandler) RegisterSchool(c *gin.Context) {
	var req dto.SchoolRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()

	existing, err := database.Client.SchoolDetails.FindUnique(
		db.SchoolDetails.Email.Equals(req.Email),
	).Exec(ctx)

	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, dto.APIError{Success: false, Error: "School with this email already exists"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to hash password"})
		return
	}

	var schoolCode string
	for i := 0; i < 5; i++ {
		code := GenerateSchoolCode()
		chk, _ := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.SchoolCode.Equals(code),
		).Exec(ctx)
		if chk == nil {
			schoolCode = code
			break
		}
	}
	if schoolCode == "" {
		schoolCode = GenerateSchoolCode()
	}

	phoneNum := req.Phone
	if phoneNum == "" {
		phoneNum = req.SchoolMobile
	}

	activationToken := uuid.New().String()
	activationExpiry := time.Now().Add(24 * time.Hour)

	newSchool, err := database.Client.SchoolDetails.CreateOne(
		db.SchoolDetails.SchoolCode.Set(schoolCode),
		db.SchoolDetails.SchoolName.Set(req.SchoolName),
		db.SchoolDetails.Email.Set(req.Email),
		db.SchoolDetails.Password.Set(string(hashedPassword)),
		db.SchoolDetails.Board.Set(req.Board),
		db.SchoolDetails.State.Set(req.State),
		db.SchoolDetails.District.Set(req.District),
		db.SchoolDetails.City.Set(req.City),
		db.SchoolDetails.Address.Set(req.Address),
		db.SchoolDetails.Pincode.Set(req.Pincode),
		db.SchoolDetails.Phone.Set(phoneNum),
		db.SchoolDetails.PrincipalName.Set(req.PrincipalName),
		db.SchoolDetails.CoordinatorName.Set(req.CoordinatorName),
		db.SchoolDetails.CoordinatorDesignation.Set(req.CoordinatorDesignation),
		db.SchoolDetails.CoordinatorMobile.Set(req.CoordinatorMobile),
		db.SchoolDetails.CoordinatorEmail.Set(req.CoordinatorEmail),
		db.SchoolDetails.IsActivated.Set(false),
		db.SchoolDetails.IsVerified.Set(false),
		db.SchoolDetails.ActivationToken.Set(activationToken),
		db.SchoolDetails.ActivationExpiry.Set(activationExpiry),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to register school: " + err.Error()})
		return
	}

	// Build the primary activation link (points at the frontend SPA) and a
	// long-lived resend link (points at the backend directly, opened from
	// the email itself, in case the 24h activation link has expired).
	activationLink := fmt.Sprintf("%s/activation-pending?token=%s&type=school", h.cfg.FrontendURL, activationToken)
	resendLink := h.buildResendLink(req.Email)

	// Send activation email link asynchronously in background goroutine so registration response is instant
	go func(toEmail, schoolName, actLink, resLink string) {
		_ = h.zohoService.SendActivationEmail(toEmail, schoolName, actLink, resLink)
	}(req.Email, req.SchoolName, activationLink, resendLink)

	// The account isn't activated yet, so there's no session or profile to
	// hand back — just the email, for the Activation Pending page.
	c.JSON(http.StatusCreated, dto.RegisterPendingResponse{
		Success: true,
		Message: "School registered successfully. An activation link has been sent to your email. Please click the link to activate your account before logging in.",
		Email:   newSchool.Email,
	})
}

// LoginSchool authenticates school by email/password. Fails if isActivated
// is false. On success it does NOT issue a session yet — instead it sends a
// one-time code to the school's email. The caller must then complete
// /api/auth/verify-otp (mode=mail or mode=msauth) before a session cookie
// is set.
func (h *AuthHandler) LoginSchool(c *gin.Context) {
	var req dto.SchoolLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	school, err := database.Client.SchoolDetails.FindUnique(
		db.SchoolDetails.Email.Equals(req.Email),
	).Exec(ctx)

	if err != nil || school == nil {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(school.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Invalid email or password"})
		return
	}

	// Block login attempt if account is not activated
	if !school.IsActivated {
		c.JSON(http.StatusForbidden, dto.APIError{
			Success: false,
			Error:   "Account is not activated. Please activate your account using the activation link sent to your email.",
		})
		return
	}

	otpCode := GenerateNumericOTP()
	otpExpiry := time.Now().Add(10 * time.Minute)

	_, err = database.Client.SchoolDetails.FindUnique(
		db.SchoolDetails.ID.Equals(school.ID),
	).Update(
		db.SchoolDetails.Otp.Set(otpCode),
		db.SchoolDetails.OtpExpiry.Set(otpExpiry),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to prepare verification code"})
		return
	}

	if err := h.zohoService.SendOTPEmail(school.Email, school.SchoolName, otpCode); err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to send verification email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.LoginPendingResponse{
		Success: true,
		Message: "Login credentials verified. A verification code has been sent to your email — please complete verification to continue.",
		Email:   school.Email,
	})
}

// Logout clears auth cookies, resets session by userEmail, and creates a SessionLog entry
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	userEmail := req.Email
	if userEmail == "" {
		userEmail = c.Query("email")
	}

	ctx := context.Background()
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	cookieToken, err := c.Cookie(middleware.JWTCookieName)
	if err != nil || cookieToken == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			cookieToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if cookieToken != "" {
		claims, err := middleware.ValidateJWT(cookieToken, h.cfg.JWTSecret)
		if err == nil && claims != nil {
			if userEmail == "" {
				userEmail = claims.Email
			}
			if claims.Role == "school" {
				_, _ = database.Client.SchoolDetails.FindUnique(
					db.SchoolDetails.ID.Equals(claims.UserID),
				).Update(
					db.SchoolDetails.IsVerified.Set(false),
					db.SchoolDetails.SessionID.Set(""),
					db.SchoolDetails.CsrfID.Set(""),
					db.SchoolDetails.RememberMe.Set(false),
				).Exec(ctx)
			} else {
				_, _ = database.Client.User.FindUnique(
					db.User.ID.Equals(claims.UserID),
				).Update(
					db.User.IsVerified.Set(false),
					db.User.SessionID.Set(""),
					db.User.CsrfID.Set(""),
					db.User.RememberMe.Set(false),
				).Exec(ctx)
			}
		}
	}

	if userEmail != "" {
		// Deactivate active session flags by userEmail
		_, _ = database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.Email.Equals(userEmail),
		).Update(
			db.SchoolDetails.IsVerified.Set(false),
			db.SchoolDetails.SessionID.Set(""),
			db.SchoolDetails.CsrfID.Set(""),
			db.SchoolDetails.RememberMe.Set(false),
		).Exec(ctx)

		_, _ = database.Client.User.FindUnique(
			db.User.Email.Equals(userEmail),
		).Update(
			db.User.IsVerified.Set(false),
			db.User.SessionID.Set(""),
			db.User.CsrfID.Set(""),
			db.User.RememberMe.Set(false),
		).Exec(ctx)

		// Record SessionLog logout entry
		now := time.Now()
		_, _ = database.Client.SessionLog.CreateOne(
			db.SessionLog.UserEmail.Set(userEmail),
			db.SessionLog.Action.Set("LOGOUT"),
			db.SessionLog.IsActive.Set(false),
			db.SessionLog.LogoutAt.Set(now),
			db.SessionLog.IPAddress.Set(clientIP),
			db.SessionLog.UserAgent.Set(userAgent),
		).Exec(ctx)
	}

	c.SetCookie(middleware.JWTCookieName, "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
	c.SetCookie(middleware.CSRFCookieName, "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, false)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}

// GetMe retrieves profile for School or User depending on JWT context
func (h *AuthHandler) GetMe(c *gin.Context) {
	userIDVal, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Unauthorized context"})
		return
	}
	userID := userIDVal.(string)

	userRoleVal, _ := c.Get("userRole")
	userRole, _ := userRoleVal.(string)

	ctx := context.Background()

	if userRole == "school" {
		school, err := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.ID.Equals(userID),
		).Exec(ctx)

		if err != nil || school == nil {
			c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "School profile not found"})
			return
		}

		board, _ := school.Board()
		state, _ := school.State()
		district, _ := school.District()
		city, _ := school.City()
		address, _ := school.Address()
		pincode, _ := school.Pincode()
		phone, _ := school.Phone()
		principal, _ := school.PrincipalName()
		coordName, _ := school.CoordinatorName()
		coordDesig, _ := school.CoordinatorDesignation()
		coordMob, _ := school.CoordinatorMobile()
		coordEmail, _ := school.CoordinatorEmail()
		cID, _ := school.CsrfID()
		sID, _ := school.SessionID()
		ip, _ := school.IPAddress()
		sExp := parseSessionExpiry(school.SessionExpiry())

		c.JSON(http.StatusOK, dto.SchoolResponse{
			ID:                     school.ID,
			SchoolCode:             school.SchoolCode,
			SchoolName:             school.SchoolName,
			Email:                  school.Email,
			Board:                  board,
			State:                  state,
			District:               district,
			City:                   city,
			Address:                address,
			Pincode:                pincode,
			Phone:                  phone,
			PrincipalName:          principal,
			CoordinatorName:        coordName,
			CoordinatorDesignation: coordDesig,
			CoordinatorMobile:      coordMob,
			CoordinatorEmail:       coordEmail,
			TwoFactorEnable:        school.TwoFactorEnable,
			IsActivated:            school.IsActivated,
			IsVerified:             school.IsVerified,
			CSRFID:                 cID,
			SessionID:              sID,
			IPAddress:              ip,
			RememberMe:             school.RememberMe,
			SessionExpiry:          sExp,
			CreatedAt:              school.CreatedAt,
			UpdatedAt:              school.UpdatedAt,
		})
		return
	}

	// User Profile Lookup
	user, err := database.Client.User.FindUnique(
		db.User.ID.Equals(userID),
	).Exec(ctx)

	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "User profile not found"})
		return
	}

	cID, _ := user.CsrfID()
	sID, _ := user.SessionID()
	ip, _ := user.IPAddress()
	sExp := parseSessionExpiry(user.SessionExpiry())

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:              user.ID,
		Name:            user.Name,
		Email:           user.Email,
		Role:            string(user.Role),
		TwoFactorEnable: user.TwoFactorEnable,
		IsActivated:     user.IsActivated,
		IsVerified:      user.IsVerified,
		CSRFID:          cID,
		SessionID:       sID,
		IPAddress:       ip,
		RememberMe:      user.RememberMe,
		SessionExpiry:   sExp,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	})
}

// SubmitContactEnquiry handles public contact messages and forwards them to support@technikolympiad.com
func (h *AuthHandler) SubmitContactEnquiry(c *gin.Context) {
	var req dto.ContactEnquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Please fill in all required fields (Name, Email, Subject, and Message).",
		})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	subject := strings.TrimSpace(req.Subject)
	message := strings.TrimSpace(req.Message)
	phone := strings.TrimSpace(req.Phone)

	if name == "" || email == "" || subject == "" || message == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Name, Email, Subject, and Message cannot be empty.",
		})
		return
	}

	// Send email asynchronously so user doesn't wait
	go func() {
		if err := h.zohoService.SendContactEnquiryEmail(name, email, phone, subject, message); err != nil {
			logger.Error("Failed to deliver contact inquiry email for %s: %v", email, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Your message has been sent successfully. We will get back to you shortly.",
	})
}

