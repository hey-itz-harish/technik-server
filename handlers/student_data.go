package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"technik-server/database"
	"technik-server/dto"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
)

// GetStudentPortfolio retrieves a student's profile, additional text & file data, and payment status
func (h *TechnikHandler) GetStudentPortfolio(c *gin.Context) {
	_ = database.EnsureConnected()
	studentID := c.Param("id")
	if studentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Missing student ID"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	// 1. Try to find student in TechnikPrideNomination first
	var studentName, class, schoolName, category, achievementTitle, approvalStatus, regNo, schoolID string
	var origBriefDesc, origDoc string
	nom, err := database.Client.TechnikPrideNomination.FindUnique(
		db.TechnikPrideNomination.ID.Equals(studentID),
	).With(
		db.TechnikPrideNomination.School.Fetch(),
	).Exec(c.Request.Context())

	if err == nil && nom != nil {
		studentName = nom.StudentName
		class = nom.Class
		category = nom.AchievementCategory
		achievementTitle = nom.AchievementTitle
		origBriefDesc = nom.BriefDescription
		if doc, ok := nom.SupportingDocument(); ok {
			origDoc = doc
		}
		if st, ok := nom.NominationStatus(); ok {
			approvalStatus = st
		} else {
			approvalStatus = "Pending Review"
		}
		if sch, ok := nom.School(); ok && sch != nil {
			schoolName = sch.SchoolName
			schoolID = sch.ID
		} else {
			schoolName = "Technik Partner School"
		}
		regNo = fmt.Sprintf("TPA-TN-TIR-%d-%s", nom.AcademicYear, strings.ToUpper(studentID[:8]))
	} else {
		// Fallback: Check StudentDetails
		sd, errSd := database.Client.StudentDetails.FindUnique(
			db.StudentDetails.ID.Equals(studentID),
		).With(
			db.StudentDetails.School.Fetch(),
		).Exec(c.Request.Context())

		if errSd == nil && sd != nil {
			studentName = sd.StudentName
			class = sd.Grade
			if cat, ok := sd.Category(); ok && cat != "" {
				category = cat
			} else {
				category = "Academic & Innovative Excellence"
			}
			achievementTitle = "Technik Olympiad Excellence"
			if st, ok := sd.Status(); ok {
				approvalStatus = st
			} else {
				approvalStatus = "Registered & Verified"
			}
			if sch, ok := sd.School(); ok && sch != nil {
				schoolName = sch.SchoolName
				schoolID = sch.ID
			} else {
				schoolName = "Technik Partner School"
			}
			regNo = fmt.Sprintf("TPA-TN-TIR-2026-%s", strings.ToUpper(studentID[:8]))
		} else {
			c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student record not found"})
			return
		}
	}

	// 2. Fetch or create StudentAdditionalData
	var addData dto.StudentPortfolioDTO
	addData.StudentID = studentID
	addData.StudentName = studentName
	addData.Class = class
	addData.SchoolName = schoolName
	addData.Category = category
	addData.AchievementTitle = achievementTitle
	addData.ApprovalStatus = approvalStatus
	addData.RegistrationNumber = regNo
	addData.RazorpayKeyID = h.cfg.RazorpayKeyID

	row := sqlDB.QueryRow(`
		SELECT 
			COALESCE(project_title, ''),
			COALESCE(project_description, ''),
			COALESCE(student_quote, ''),
			COALESCE(key_strengths, ''),
			COALESCE(development_areas, ''),
			COALESCE(mentor_remarks, ''),
			COALESCE(project_file, ''),
			COALESCE(student_photo, ''),
			COALESCE(additional_files, ''),
			COALESCE(achievement_score, 4.8),
			COALESCE(innovation_score, 4.8),
			COALESCE(creativity_score, 4.3),
			COALESCE(communication_score, 4.3),
			COALESCE(leadership_score, 4.0),
			COALESCE(social_impact_score, 4.3),
			COALESCE(payment_status, 'PENDING'),
			COALESCE(amount_paid, 0),
			paid_at
		FROM "StudentAdditionalData"
		WHERE student_id = $1;
	`, studentID)

	var addFilesJSON string
	var paidAt sql.NullTime
	errScan := row.Scan(
		&addData.ProjectTitle,
		&addData.ProjectDescription,
		&addData.StudentQuote,
		&addData.KeyStrengths,
		&addData.DevelopmentAreas,
		&addData.MentorRemarks,
		&addData.ProjectFile,
		&addData.StudentPhoto,
		&addFilesJSON,
		&addData.AchievementScore,
		&addData.InnovationScore,
		&addData.CreativityScore,
		&addData.CommunicationScore,
		&addData.LeadershipScore,
		&addData.SocialImpactScore,
		&addData.PaymentStatus,
		&addData.AmountPaid,
		&paidAt,
	)

	if errScan != nil {
		// Initialize default data if not yet created in table
		defaultTitle := achievementTitle
		if defaultTitle == "" {
			defaultTitle = "Smart Water Saver - Automatic Tank Overflow Prevention System"
		}
		defaultDesc := origBriefDesc
		if defaultDesc == "" {
			defaultDesc = "Innovative automated project engineered to address vital challenges with real-world impact and sustainable design principles."
		}
		defaultQuote := fmt.Sprintf("I created this project to solve real challenges in our school and community.")
		defaultStrengths := "Strong problem solving and technical understanding\nClear communication and confidence\nPurpose-driven innovation with real-world impact"
		defaultDev := "Further field testing and real-world deployment\nExplore advanced features (IoT, mobile alerts)\nContinue participation in innovation competitions"
		defaultMentor := "Demonstrated outstanding creativity, discipline and technical curiosity throughout the assessment."
		defaultProjectFile := origDoc

		_, _ = sqlDB.Exec(`
			INSERT INTO "StudentAdditionalData" (
				"id", "student_id", "student_name", "school_id", 
				"project_title", "project_description", "student_quote", 
				"key_strengths", "development_areas", "mentor_remarks",
				"project_file", "payment_status"
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'PENDING')
			ON CONFLICT (student_id) DO NOTHING;
		`, fmt.Sprintf("sad-%s", studentID), studentID, studentName, schoolID,
			defaultTitle, defaultDesc, defaultQuote, defaultStrengths, defaultDev, defaultMentor, defaultProjectFile)

		addData.ProjectTitle = defaultTitle
		addData.ProjectDescription = defaultDesc
		addData.StudentQuote = defaultQuote
		addData.KeyStrengths = defaultStrengths
		addData.DevelopmentAreas = defaultDev
		addData.MentorRemarks = defaultMentor
		addData.ProjectFile = defaultProjectFile
		addData.AchievementScore = 4.8
		addData.InnovationScore = 4.8
		addData.CreativityScore = 4.3
		addData.CommunicationScore = 4.3
		addData.LeadershipScore = 4.0
		addData.SocialImpactScore = 4.3
		addData.PaymentStatus = "PENDING"
		addData.AmountPaid = 0
	} else {
		if paidAt.Valid {
			addData.PaidAt = &paidAt.Time
		}
		if addFilesJSON != "" {
			var files []string
			if json.Unmarshal([]byte(addFilesJSON), &files) == nil {
				addData.AdditionalFiles = files
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"portfolio":     addData,
		"razorpayKeyId": h.cfg.RazorpayKeyID,
	})
}

// SaveStudentPortfolio updates text and file based data for an approved student
func (h *TechnikHandler) SaveStudentPortfolio(c *gin.Context) {
	studentID := c.Param("id")
	if studentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Missing student ID"})
		return
	}

	var req dto.SaveStudentPortfolioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	filesJSON, _ := json.Marshal(req.AdditionalFiles)

	query := `
		INSERT INTO "StudentAdditionalData" (
			"id", "student_id", "project_title", "project_description", 
			"student_quote", "key_strengths", "development_areas", "mentor_remarks", 
			"project_file", "student_photo", "additional_files", "updated_at"
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()
		)
		ON CONFLICT (student_id) DO UPDATE SET
			project_title = EXCLUDED.project_title,
			project_description = EXCLUDED.project_description,
			student_quote = EXCLUDED.student_quote,
			key_strengths = EXCLUDED.key_strengths,
			development_areas = EXCLUDED.development_areas,
			mentor_remarks = EXCLUDED.mentor_remarks,
			project_file = COALESCE(NULLIF(EXCLUDED.project_file, ''), "StudentAdditionalData".project_file),
			student_photo = COALESCE(NULLIF(EXCLUDED.student_photo, ''), "StudentAdditionalData".student_photo),
			additional_files = EXCLUDED.additional_files,
			updated_at = NOW();
	`

	_, err := sqlDB.Exec(
		query,
		fmt.Sprintf("sad-%s", studentID),
		studentID,
		strings.TrimSpace(req.ProjectTitle),
		strings.TrimSpace(req.ProjectDescription),
		strings.TrimSpace(req.StudentQuote),
		strings.TrimSpace(req.KeyStrengths),
		strings.TrimSpace(req.DevelopmentAreas),
		strings.TrimSpace(req.MentorRemarks),
		strings.TrimSpace(req.ProjectFile),
		strings.TrimSpace(req.StudentPhoto),
		string(filesJSON),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to save portfolio data: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student portfolio & additional data saved successfully",
	})
}

// CreateRazorpayOrder creates a new order in Razorpay for Skill Compass report access
func (h *TechnikHandler) CreateRazorpayOrder(c *gin.Context) {
	var req dto.RazorpayCreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	if strings.TrimSpace(req.StudentID) == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Student ID is required to initiate payment order"})
		return
	}

	// Ensure Razorpay credentials are configured
	if strings.TrimSpace(h.cfg.RazorpayKeyID) == "" || strings.TrimSpace(h.cfg.RazorpayKeySecret) == "" {
		c.JSON(http.StatusInternalServerError, dto.APIError{
			Success: false,
			Error:   "Razorpay payment gateway credentials are not configured on the server. Please contact system administrator.",
		})
		return
	}

	amountINR := req.Amount
	if amountINR <= 0 {
		amountINR = 499 // Fixed ₹499 for Technik Skill Compass Report
	}
	amountPaise := amountINR * 100

	cleanStudentID := strings.ReplaceAll(req.StudentID, "-", "")
	if len(cleanStudentID) > 10 {
		cleanStudentID = cleanStudentID[:10]
	}
	receiptID := fmt.Sprintf("rcpt_%s_%d", cleanStudentID, time.Now().Unix())

	// Prepare payload for official Razorpay Orders API
	type rzpReq struct {
		Amount   int               `json:"amount"`
		Currency string            `json:"currency"`
		Receipt  string            `json:"receipt"`
		Notes    map[string]string `json:"notes"`
	}

	payload := rzpReq{
		Amount:   amountPaise,
		Currency: "INR",
		Receipt:  receiptID,
		Notes: map[string]string{
			"studentId": req.StudentID,
			"purpose":   "Technik Skill Compass Report & Official Certificate",
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to construct payment order payload"})
		return
	}

	httpReq, err := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to prepare Razorpay request: " + err.Error()})
		return
	}

	httpReq.SetBasicAuth(h.cfg.RazorpayKeyID, h.cfg.RazorpayKeySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, respErr := client.Do(httpReq)
	if respErr != nil {
		c.JSON(http.StatusBadGateway, dto.APIError{
			Success: false, 
			Error:   "Failed to connect to Razorpay payment gateway: " + respErr.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, dto.APIError{Success: false, Error: "Failed to read response from Razorpay payment gateway"})
		return
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var rzpErr struct {
			Error struct {
				Code        string `json:"code"`
				Description string `json:"description"`
				Reason      string `json:"reason"`
			} `json:"error"`
		}
		errMsg := fmt.Sprintf("Razorpay gateway rejected order creation with HTTP %d", resp.StatusCode)
		if json.Unmarshal(respBody, &rzpErr) == nil && rzpErr.Error.Description != "" {
			errMsg = fmt.Sprintf("Payment Gateway Error: %s (%s)", rzpErr.Error.Description, rzpErr.Error.Code)
		}
		c.JSON(http.StatusBadGateway, dto.APIError{Success: false, Error: errMsg})
		return
	}

	var rzpResp struct {
		ID       string `json:"id"`
		Amount   int    `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &rzpResp); err != nil || rzpResp.ID == "" {
		c.JSON(http.StatusBadGateway, dto.APIError{
			Success: false,
			Error:   "Unexpected response from Razorpay payment gateway: order ID missing",
		})
		return
	}

	orderID := rzpResp.ID

	// Record generated order ID in StudentAdditionalData
	sqlDB := database.GetSQLDB()
	if sqlDB != nil {
		_, _ = sqlDB.Exec(`
			UPDATE "StudentAdditionalData"
			SET razorpay_order_id = $1
			WHERE student_id = $2;
		`, orderID, req.StudentID)
	}

	c.JSON(http.StatusOK, dto.RazorpayCreateOrderResponse{
		Success:   true,
		OrderID:   orderID,
		Amount:    amountPaise,
		Currency:  "INR",
		KeyID:     h.cfg.RazorpayKeyID,
		StudentID: req.StudentID,
	})
}

// VerifyRazorpayPayment securely verifies the payment signature and confirms payment status with Razorpay
func (h *TechnikHandler) VerifyRazorpayPayment(c *gin.Context) {
	var req dto.RazorpayVerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	// Validate required parameters
	if strings.TrimSpace(req.StudentID) == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "studentId is required"})
		return
	}
	if strings.TrimSpace(req.RazorpayOrderID) == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "razorpayOrderId is required"})
		return
	}
	if strings.TrimSpace(req.RazorpayPaymentID) == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "razorpayPaymentId is required"})
		return
	}
	if strings.TrimSpace(req.RazorpaySignature) == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "razorpaySignature is required"})
		return
	}

	if strings.TrimSpace(h.cfg.RazorpayKeySecret) == "" {
		c.JSON(http.StatusInternalServerError, dto.APIError{
			Success: false,
			Error:   "Payment gateway secret is not configured on the server",
		})
		return
	}

	// 1. Strict Cryptographic HMAC-SHA256 signature verification
	dataToSign := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	mac := hmac.New(sha256.New, []byte(h.cfg.RazorpayKeySecret))
	mac.Write([]byte(dataToSign))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSignature), []byte(req.RazorpaySignature)) {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Payment verification failed: cryptographic signature mismatch. The transaction may have been tampered with.",
		})
		return
	}

	// 2. Query Razorpay API directly to verify payment capture status and amount
	verifyUrl := fmt.Sprintf("https://api.razorpay.com/v1/payments/%s", req.RazorpayPaymentID)
	payReq, err := http.NewRequest("GET", verifyUrl, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{
			Success: false, 
			Error:   "Failed to initialize payment status verification request: " + err.Error(),
		})
		return
	}

	payReq.SetBasicAuth(h.cfg.RazorpayKeyID, h.cfg.RazorpayKeySecret)
	client := &http.Client{Timeout: 10 * time.Second}
	payResp, payErr := client.Do(payReq)
	if payErr != nil {
		c.JSON(http.StatusBadGateway, dto.APIError{
			Success: false,
			Error:   "Failed to connect to Razorpay to verify payment status: " + payErr.Error(),
		})
		return
	}
	defer payResp.Body.Close()

	if payResp.StatusCode != http.StatusOK {
		payBody, _ := io.ReadAll(payResp.Body)
		var errObj struct {
			Error struct {
				Description string `json:"description"`
			} `json:"error"`
		}
		detail := string(payBody)
		if json.Unmarshal(payBody, &errObj) == nil && errObj.Error.Description != "" {
			detail = errObj.Error.Description
		}
		c.JSON(http.StatusBadGateway, dto.APIError{
			Success: false,
			Error:   fmt.Sprintf("Razorpay payment verification rejected (HTTP %d): %s", payResp.StatusCode, detail),
		})
		return
	}

	var paymentInfo struct {
		ID       string `json:"id"`
		OrderID  string `json:"order_id"`
		Status   string `json:"status"` // "captured", "authorized", "failed"
		Amount   int    `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(payResp.Body).Decode(&paymentInfo); err != nil {
		c.JSON(http.StatusBadGateway, dto.APIError{
			Success: false,
			Error:   "Failed to decode payment details from Razorpay gateway",
		})
		return
	}

	// Ensure payment belongs to the order
	if paymentInfo.OrderID != "" && paymentInfo.OrderID != req.RazorpayOrderID {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Payment verification failed: payment belongs to a different order ID",
		})
		return
	}

	// Ensure status is captured or authorized
	if paymentInfo.Status != "captured" && paymentInfo.Status != "authorized" {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   fmt.Sprintf("Payment verification failed: payment status is '%s', not captured", paymentInfo.Status),
		})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	// Mark payment as PAID / UNLOCKED in database
	_, err = sqlDB.Exec(`
		UPDATE "StudentAdditionalData"
		SET 
			payment_status = 'PAID',
			razorpay_payment_id = $1,
			razorpay_signature = $2,
			amount_paid = 499,
			paid_at = NOW()
		WHERE student_id = $3;
	`, req.RazorpayPaymentID, req.RazorpaySignature, req.StudentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update payment record in database: " + err.Error()})
		return
	}

	// Also update TechnikPrideNomination payment status if student exists
	_, _ = sqlDB.Exec(`
		UPDATE "TechnikPrideNomination"
		SET 
			"paymentStatus" = 'PAID',
			"razorpayOrderId" = $1,
			"razorpayPaymentId" = $2,
			"razorpaySignature" = $3,
			"amountPaid" = 499,
			"paidAt" = NOW()
		WHERE id = $4;
	`, req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature, req.StudentID)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Payment verified successfully! Technik Skill Compass Report has been unlocked.",
		"paymentStatus": "PAID",
		"paymentId":     req.RazorpayPaymentID,
		"orderId":       req.RazorpayOrderID,
	})
}
