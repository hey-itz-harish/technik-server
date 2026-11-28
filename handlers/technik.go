package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TechnikHandler struct {
	cfg *config.Config
}

func NewTechnikHandler(cfg *config.Config) *TechnikHandler {
	return &TechnikHandler{cfg: cfg}
}

// Helper to determine if a class is Junior (Grades 3-5) or Senior (Grades 6-8)
func isJuniorLevel(classStr string) bool {
	combined := strings.ToLower(classStr)
	if strings.Contains(combined, "jr") ||
		strings.Contains(combined, "junior") ||
		strings.Contains(combined, "grade 3") ||
		strings.Contains(combined, "grade 4") ||
		strings.Contains(combined, "grade 5") ||
		strings.Contains(combined, "class 3") ||
		strings.Contains(combined, "class 4") ||
		strings.Contains(combined, "class 5") ||
		strings.Contains(combined, "class iii") ||
		strings.Contains(combined, "class iv") ||
		strings.Contains(combined, "class v") {
		return true
	}
	return false
}

// UploadDocument handles multipart file upload for nominations / certificates with strict validation
func (h *TechnikHandler) UploadDocument(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "No file uploaded or invalid form key 'file'"})
		return
	}

	// 1. Enforce max file size: 10 MB
	const maxUploadSize = 10 * 1024 * 1024
	if file.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Uploaded file exceeds maximum limit of 10MB"})
		return
	}

	// 2. Validate file extension against allowed document/image types
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExtensions := map[string]bool{
		".pdf":  true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	if !allowedExtensions[ext] {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Invalid file extension. Only PDF, JPG, PNG, and WEBP documents are allowed.",
		})
		return
	}

	// 3. Inspect magic bytes / MIME type to prevent disguised scripts
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to open uploaded file for validation"})
		return
	}
	defer src.Close()

	buffer := make([]byte, 512)
	n, _ := src.Read(buffer)
	mimeType := http.DetectContentType(buffer[:n])

	allowedMIMEs := map[string]bool{
		"application/pdf": true,
		"image/jpeg":      true,
		"image/png":       true,
		"image/webp":      true,
	}
	if !allowedMIMEs[mimeType] {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Invalid file format. File content does not match allowed document/image formats.",
		})
		return
	}

	// 4. Create directory if not exists with restricted permissions
	uploadDir := "./uploads/nominations"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create upload directory"})
		return
	}

	cleanFileName := filepath.Base(file.Filename)
	cleanFileName = strings.ReplaceAll(cleanFileName, " ", "_")
	timestamp := time.Now().UnixNano()
	savedFileName := fmt.Sprintf("%d_%s", timestamp, cleanFileName)
	targetPath := filepath.Join(uploadDir, savedFileName)

	if err := c.SaveUploadedFile(file, targetPath); err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to save file on server"})
		return
	}

	relativeURL := fmt.Sprintf("/uploads/nominations/%s", savedFileName)
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Document uploaded successfully",
		"fileName": file.Filename,
		"filePath": relativeURL,
	})
}

// NominateTechnikPride handles student nominations for Technik Pride Award with Quota Check:
// Max 3 students for Junior Level (Grades 3-5) and 3 students for Senior Level (Grades 6-8) per school / year (Total 6).
// Also syncs nominated students into StudentDetails schema with category "Technik Pride Award".
func (h *TechnikHandler) NominateTechnikPride(c *gin.Context) {
	ctx := context.Background()
	currentYear := time.Now().Year()

	// Extract schoolId from JWT context if authenticated
	var sessionSchoolID string
	if val, exists := c.Get("userId"); exists {
		if roleVal, roleExists := c.Get("userRole"); roleExists && roleVal == "school" {
			sessionSchoolID = val.(string)
		}
	}

	// Try reading as batch nomination request
	var batchReq dto.TechnikPrideBatchNominationRequest
	if err := c.ShouldBindJSON(&batchReq); err == nil && len(batchReq.Nominations) > 0 {
		year := batchReq.AcademicYear
		if year <= 0 {
			year = currentYear
		}

		userRole, _ := c.Get("userRole")
		targetSchoolID := batchReq.SchoolID
		if userRole == "school" || targetSchoolID == "" {
			targetSchoolID = sessionSchoolID
		}

		if targetSchoolID == "" {
			c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "School identification is required"})
			return
		}

		// Fetch existing nominations for this school and year to check quota
		var existingNoms []db.TechnikPrideNominationModel
		if targetSchoolID != "" {
			existingNoms, _ = database.Client.TechnikPrideNomination.FindMany(
				db.TechnikPrideNomination.AcademicYear.Equals(year),
				db.TechnikPrideNomination.SchoolID.Equals(targetSchoolID),
			).Exec(ctx)
		}

		existingCount := len(existingNoms)
		incomingCount := len(batchReq.Nominations)

		// Enforce Quota: Max 8 nominations per school per academic year (across 4 tiers: Foundation, Preparatory, Aspire, Aspire Plus)
		if existingCount+incomingCount > 8 {
			c.JSON(http.StatusBadRequest, dto.APIError{
				Success: false,
				Error: fmt.Sprintf("Annual Quota Limit Exceeded: Each school can nominate a maximum of 8 students per academic year (up to 2 students across each of the 4 categories: Foundation, Preparatory, Aspire, Aspire Plus). You have %d registered and attempted to add %d more.", existingCount, incomingCount),
			})
			return
		}

		var createdResponses []dto.TechnikPrideNominationResponse
		for _, nom := range batchReq.Nominations {
			schoolID := nom.SchoolID
			if schoolID == "" {
				schoolID = targetSchoolID
			}

			params := []db.TechnikPrideNominationSetParam{
				db.TechnikPrideNomination.AcademicYear.Set(year),
				db.TechnikPrideNomination.SupportingDocument.Set(nom.SupportingDocument),
				db.TechnikPrideNomination.NominationStatus.Set("Submitted & Under Review"),
				db.TechnikPrideNomination.AdminStatus.Set("Forwarded to Technik Super Admin"),
			}
			if schoolID != "" {
				params = append(params, db.TechnikPrideNomination.School.Link(
					db.SchoolDetails.ID.Equals(schoolID),
				))
			}

			res, err := database.Client.TechnikPrideNomination.CreateOne(
				db.TechnikPrideNomination.StudentName.Set(nom.StudentName),
				db.TechnikPrideNomination.Class.Set(nom.Class),
				db.TechnikPrideNomination.Gender.Set(nom.Gender),
				db.TechnikPrideNomination.AchievementCategory.Set(nom.AchievementCategory),
				db.TechnikPrideNomination.AchievementTitle.Set(nom.AchievementTitle),
				db.TechnikPrideNomination.BriefDescription.Set(nom.BriefDescription),
				params...,
			).Exec(ctx)

			if err == nil && res != nil {
				schID, _ := res.SchoolID()
				supDoc, _ := res.SupportingDocument()
				nomStat, _ := res.NominationStatus()
				admStat, _ := res.AdminStatus()

				if nom.StudentID != "" {
					if sqlDB := database.GetSQLDB(); sqlDB != nil {
						_, _ = sqlDB.Exec(`UPDATE "TechnikPrideNomination" SET "studentId" = $1 WHERE id = $2;`, nom.StudentID, res.ID)
					}
				}

				createdResponses = append(createdResponses, dto.TechnikPrideNominationResponse{
					ID:                  res.ID,
					SchoolID:            schID,
					StudentID:           nom.StudentID,
					AcademicYear:        res.AcademicYear,
					StudentName:         res.StudentName,
					Class:               res.Class,
					Gender:              res.Gender,
					AchievementCategory: res.AchievementCategory,
					AchievementTitle:    res.AchievementTitle,
					BriefDescription:    res.BriefDescription,
					SupportingDocument:  supDoc,
					NominationStatus:    nomStat,
					AdminStatus:         admStat,
					CreatedAt:           res.CreatedAt,
					UpdatedAt:           res.UpdatedAt,
				})

				// Also add student into StudentDetails schema
				var stParams []db.StudentDetailsSetParam
				stParams = append(stParams,
					db.StudentDetails.Gender.Set(nom.Gender),
					db.StudentDetails.AcademicYear.Set(year),
				)
				if schoolID != "" {
					stParams = append(stParams, db.StudentDetails.School.Link(db.SchoolDetails.ID.Equals(schoolID)))
				}

				_, _ = database.Client.StudentDetails.CreateOne(
					db.StudentDetails.StudentName.Set(nom.StudentName),
					db.StudentDetails.Grade.Set(nom.Class),
					stParams...,
				).Exec(ctx)
			}
		}

		c.JSON(http.StatusCreated, dto.TechnikPrideAuthResponse{
			Success:     true,
			Message:     "Technik Pride Award nominations submitted successfully",
			Total:       len(createdResponses),
			Nominations: createdResponses,
		})
		return
	}

	// Single nomination fallback
	var singleReq dto.TechnikPrideNominationRequest
	if err := c.ShouldBindJSON(&singleReq); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	year := singleReq.AcademicYear
	if year <= 0 {
		year = currentYear
	}

	userRole, _ := c.Get("userRole")
	targetSchoolID := singleReq.SchoolID
	if userRole == "school" || targetSchoolID == "" {
		targetSchoolID = sessionSchoolID
	}

	if targetSchoolID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "School identification is required"})
		return
	}

	// Check existing nominations for this school and year
	var existingNoms []db.TechnikPrideNominationModel
	if targetSchoolID != "" {
		existingNoms, _ = database.Client.TechnikPrideNomination.FindMany(
			db.TechnikPrideNomination.AcademicYear.Equals(year),
			db.TechnikPrideNomination.SchoolID.Equals(targetSchoolID),
		).Exec(ctx)
	}

	existingCount := len(existingNoms)
	if existingCount >= 8 {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   fmt.Sprintf("Annual Quota Limit Exceeded: Each school can nominate a maximum of 8 students per academic year across the 4 categories (currently %d registered).", existingCount),
		})
		return
	}

	params := []db.TechnikPrideNominationSetParam{
		db.TechnikPrideNomination.AcademicYear.Set(year),
		db.TechnikPrideNomination.SupportingDocument.Set(singleReq.SupportingDocument),
		db.TechnikPrideNomination.NominationStatus.Set("Submitted & Under Review"),
		db.TechnikPrideNomination.AdminStatus.Set("Forwarded to Technik Super Admin"),
	}

	if targetSchoolID != "" {
		params = append(params, db.TechnikPrideNomination.School.Link(
			db.SchoolDetails.ID.Equals(targetSchoolID),
		))
	}

	res, err := database.Client.TechnikPrideNomination.CreateOne(
		db.TechnikPrideNomination.StudentName.Set(singleReq.StudentName),
		db.TechnikPrideNomination.Class.Set(singleReq.Class),
		db.TechnikPrideNomination.Gender.Set(singleReq.Gender),
		db.TechnikPrideNomination.AchievementCategory.Set(singleReq.AchievementCategory),
		db.TechnikPrideNomination.AchievementTitle.Set(singleReq.AchievementTitle),
		db.TechnikPrideNomination.BriefDescription.Set(singleReq.BriefDescription),
		params...,
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create nomination: " + err.Error()})
		return
	}

	schID, _ := res.SchoolID()
	supDoc, _ := res.SupportingDocument()
	nomStat, _ := res.NominationStatus()
	admStat, _ := res.AdminStatus()

	if singleReq.StudentID != "" {
		if sqlDB := database.GetSQLDB(); sqlDB != nil {
			_, _ = sqlDB.Exec(`UPDATE "TechnikPrideNomination" SET "studentId" = $1 WHERE id = $2;`, singleReq.StudentID, res.ID)
		}
	}

	// Also sync into StudentDetails schema
	var stParams []db.StudentDetailsSetParam
	stParams = append(stParams,
		db.StudentDetails.Gender.Set(singleReq.Gender),
		db.StudentDetails.AcademicYear.Set(year),
	)
	if targetSchoolID != "" {
		stParams = append(stParams, db.StudentDetails.School.Link(db.SchoolDetails.ID.Equals(targetSchoolID)))
	}

	_, _ = database.Client.StudentDetails.CreateOne(
		db.StudentDetails.StudentName.Set(singleReq.StudentName),
		db.StudentDetails.Grade.Set(singleReq.Class),
		stParams...,
	).Exec(ctx)

	c.JSON(http.StatusCreated, dto.TechnikPrideAuthResponse{
		Success: true,
		Message: "Technik Pride Award nomination created successfully",
		Total:   1,
		Nominations: []dto.TechnikPrideNominationResponse{
			{
				ID:                  res.ID,
				SchoolID:            schID,
				AcademicYear:        res.AcademicYear,
				StudentName:         res.StudentName,
				Class:               res.Class,
				Gender:              res.Gender,
				AchievementCategory: res.AchievementCategory,
				AchievementTitle:    res.AchievementTitle,
				BriefDescription:    res.BriefDescription,
				SupportingDocument:  supDoc,
				NominationStatus:    nomStat,
				AdminStatus:         admStat,
				CreatedAt:           res.CreatedAt,
				UpdatedAt:           res.UpdatedAt,
			},
		},
	})
}

// GetTechnikPrideNominations fetches nominations for current year or filtered query
func (h *TechnikHandler) GetTechnikPrideNominations(c *gin.Context) {
	_ = database.EnsureConnected()
	ctx := context.Background()
	yearStr := c.Query("year")
	schoolID := c.Query("schoolId")
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	userRole, _ := c.Get("userRole")
	if userRole == "school" {
		schoolID = c.GetString("userId")
	} else if schoolID == "" {
		if val, exists := c.Get("userId"); exists {
			schoolID = val.(string)
		}
	}

	year := time.Now().Year()
	hasYearFilter := false
	if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
		year = y
		hasYearFilter = true
	}

	var conditions []db.TechnikPrideNominationWhereParam
	if hasYearFilter {
		conditions = append(conditions, db.TechnikPrideNomination.AcademicYear.Equals(year))
	}
	if schoolID != "" {
		conditions = append(conditions, db.TechnikPrideNomination.SchoolID.Equals(schoolID))
	}

	list, err := database.Client.TechnikPrideNomination.FindMany(
		conditions...,
	).OrderBy(
		db.TechnikPrideNomination.CreatedAt.Order(db.SortOrderDesc),
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch nominations: " + err.Error()})
		return
	}

	var responses []dto.TechnikPrideNominationResponse
	for _, res := range list {
		schID, _ := res.SchoolID()
		clsCat, _ := res.ClassCategory()
		supDoc, _ := res.SupportingDocument()
		nomStat, _ := res.NominationStatus()
		if nomStat == "" {
			nomStat = "Submitted & Under Review"
		}
		admStat, _ := res.AdminStatus()
		if admStat == "" {
			admStat = "Forwarded to Technik Super Admin"
		}

		if search != "" {
			sName := strings.ToLower(res.StudentName)
			sCat := strings.ToLower(res.AchievementCategory)
			sTitle := strings.ToLower(res.AchievementTitle)
			if !strings.Contains(sName, search) && !strings.Contains(sCat, search) && !strings.Contains(sTitle, search) {
				continue
			}
		}

		responses = append(responses, dto.TechnikPrideNominationResponse{
			ID:                  res.ID,
			SchoolID:            schID,
			AcademicYear:        res.AcademicYear,
			StudentName:         res.StudentName,
			Class:               res.Class,
			ClassCategory:       clsCat,
			Gender:              res.Gender,
			AchievementCategory: res.AchievementCategory,
			AchievementTitle:    res.AchievementTitle,
			BriefDescription:    res.BriefDescription,
			SupportingDocument:  supDoc,
			NominationStatus:    nomStat,
			AdminStatus:         admStat,
			CreatedAt:           res.CreatedAt,
			UpdatedAt:           res.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, dto.TechnikPrideAuthResponse{
		Success:     true,
		Message:     "Nominations retrieved successfully",
		Total:       len(responses),
		Nominations: responses,
	})
}

// GetSchoolStudents lists all students enrolled in the school from TechnikPrideNomination and OlympiadRegistration/OlympiadStudent models with search & filters
func (h *TechnikHandler) GetSchoolStudents(c *gin.Context) {
	_ = database.EnsureConnected()
	ctx := context.Background()
	var schoolID string
	userRole, _ := c.Get("userRole")
	if userRole == "school" {
		schoolID = c.GetString("userId")
	} else {
		schoolID = c.Query("schoolId")
		if schoolID == "" {
			if val, exists := c.Get("userId"); exists {
				schoolID = val.(string)
			}
		}
	}

	if schoolID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "schoolId parameter is required"})
		return
	}

	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	gradeLevel := strings.TrimSpace(c.Query("gradeLevel")) // "Jr Level" or "Sr Level"
	yearStr := strings.TrimSpace(c.Query("year"))          // "2026", "2025" or "All"

	var combined []dto.StudentResponse

	// 1. Fetch Technik Pride Nominations
	var prideWhere []db.TechnikPrideNominationWhereParam
	if schoolID != "" {
		prideWhere = append(prideWhere, db.TechnikPrideNomination.SchoolID.Equals(schoolID))
	}
	prideList, err := database.Client.TechnikPrideNomination.FindMany(
		prideWhere...,
	).OrderBy(
		db.TechnikPrideNomination.CreatedAt.Order(db.SortOrderDesc),
	).Exec(ctx)

	if err == nil {
		for _, nom := range prideList {
			schID, _ := nom.SchoolID()
			clsCat, _ := nom.ClassCategory()
			if clsCat == "" {
				clsCat = "Technik Pride Award"
			}
			nomStat, _ := nom.NominationStatus()
			if nomStat == "" {
				nomStat = "Nomination Received"
			}

			category := nom.AchievementCategory
			if category == "" {
				category = "Technik Pride Award"
			}

			displayGrade := nom.Class

			combined = append(combined, dto.StudentResponse{
				ID:            nom.ID,
				StudentName:   nom.StudentName,
				Grade:         displayGrade,
				ClassCategory: clsCat,
				Category:      category,
				Gender:        nom.Gender,
				Status:        nomStat,
				AcademicYear:  nom.AcademicYear,
				SchoolID:      schID,
				CreatedAt:     nom.CreatedAt,
				UpdatedAt:     nom.UpdatedAt,
			})
		}
	}

	// 2. Fetch Olympiad Registrations from olympiadParticipants table
	if sqlDB := database.GetSQLDB(); sqlDB != nil {
		q := `SELECT op.id, op."studentName", COALESCE(op.gender, ''), COALESCE(op."olympiadTrack", 'Robotics Olympiad'),
		             COALESCE(s.grade, 'Grade 5'), op."createdAt", op."updatedAt"
		      FROM "olympiadParticipants" op
		      LEFT JOIN olympiadstudentteststatus s ON op.olympiadstudentteststatusid = s.id
		      WHERE op.deleted = FALSE`
		var rows *sql.Rows
		if schoolID != "" {
			q += ` AND (op."schoolCode" = (SELECT "schoolCode" FROM "SchoolDetails" WHERE id = $1 LIMIT 1)
			         OR op."schoolName" = (SELECT "schoolName" FROM "SchoolDetails" WHERE id = $1 LIMIT 1))`
			rows, err = sqlDB.Query(q, schoolID)
		} else {
			rows, err = sqlDB.Query(q)
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, name, gender, track, grade string
				var cAt, uAt time.Time
				if err := rows.Scan(&id, &name, &gender, &track, &grade, &cAt, &uAt); err == nil {
					combined = append(combined, dto.StudentResponse{
						ID:           id,
						StudentName:  name,
						Grade:        grade,
						Category:     track,
						Gender:       gender,
						Status:       "Registered & Verified",
						AcademicYear: 2026,
						SchoolID:     schoolID,
						CreatedAt:    cAt,
						UpdatedAt:    uAt,
					})
				}
			}
		}
	}

	// 3. Fetch direct StudentDetails if any
	var sdWhere []db.StudentDetailsWhereParam
	if schoolID != "" {
		sdWhere = append(sdWhere, db.StudentDetails.SchoolID.Equals(schoolID))
	}
	sdList, err := database.Client.StudentDetails.FindMany(
		sdWhere...,
	).OrderBy(
		db.StudentDetails.CreatedAt.Order(db.SortOrderDesc),
	).Exec(ctx)

	if err == nil {
		for _, st := range sdList {
			schID, _ := st.SchoolID()
			gender, _ := st.Gender()
			category := "Robotics Olympiad"
			status := "Registered & Verified"
			acadYear, ok := st.AcademicYear()
			if !ok || acadYear == 0 {
				acadYear = st.CreatedAt.Year()
			}

			displayGrade := st.Grade

			combined = append(combined, dto.StudentResponse{
				ID:           st.ID,
				StudentName:  st.StudentName,
				Grade:        displayGrade,
				Category:     category,
				Gender:       gender,
				Status:       status,
				AcademicYear: acadYear,
				SchoolID:     schID,
				CreatedAt:    st.CreatedAt,
				UpdatedAt:    st.UpdatedAt,
			})
		}
	}

	// 4. Filter and Deduplicate combined results
	var filtered []dto.StudentResponse
	seen := make(map[string]bool)

	for _, st := range combined {
		// Deduplicate by Name + Category + Grade if duplicate entry
		dupKey := fmt.Sprintf("%s|%s|%s", strings.ToLower(st.StudentName), strings.ToLower(st.Category), strings.ToLower(st.Grade))
		if seen[dupKey] {
			continue
		}
		seen[dupKey] = true

		// Search Filter
		if search != "" {
			nameLower := strings.ToLower(st.StudentName)
			catLower := strings.ToLower(st.Category)
			gradeLower := strings.ToLower(st.Grade)
			if !strings.Contains(nameLower, search) && !strings.Contains(catLower, search) && !strings.Contains(gradeLower, search) {
				continue
			}
		}

		// Grade Level Filter
		if gradeLevel != "" && gradeLevel != "All" {
			if gradeLevel == "Jr Level" || gradeLevel == "Junior Level" {
				if !strings.Contains(st.Grade, "Jr") && !isJuniorLevel(st.Grade) {
					continue
				}
			} else if gradeLevel == "Sr Level" || gradeLevel == "Senior Level" {
				if !strings.Contains(st.Grade, "Sr") && isJuniorLevel(st.Grade) {
					continue
				}
			}
		}

		// Academic Year Filter
		if yearStr != "" && yearStr != "All" {
			if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
				if st.AcademicYear != y {
					continue
				}
			}
		}

		filtered = append(filtered, st)
	}

	// Sort combined filtered students by CreatedAt descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	c.JSON(http.StatusOK, dto.SchoolStudentsResponse{
		Success:  true,
		Message:  "Students retrieved successfully",
		Total:    len(filtered),
		Students: filtered,
	})
}

// RegisterOlympiad handles student registration for Olympiad tracks and stores them in olympiadParticipants table
func (h *TechnikHandler) RegisterOlympiad(c *gin.Context) {
	var req dto.OlympiadRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	userRole, _ := c.Get("userRole")
	targetSchoolID := req.SchoolID
	userEmail, _ := c.Get("userEmail")
	creator := "School Portal"
	if em, ok := userEmail.(string); ok && em != "" {
		creator = em
	}

	if userRole == "school" || targetSchoolID == "" {
		if val, exists := c.Get("userId"); exists {
			targetSchoolID = val.(string)
		}
	}

	if targetSchoolID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "School identification is required"})
		return
	}

	// Fetch school details (schoolName & schoolCode)
	var schoolName, schoolCode string
	_ = sqlDB.QueryRow(`SELECT "schoolName", "schoolCode" FROM "SchoolDetails" WHERE id = $1 LIMIT 1;`, targetSchoolID).Scan(&schoolName, &schoolCode)
	if schoolName == "" {
		schoolName = "Technik Partner School"
	}

	testID := req.TestID
	now := time.Now()
	var registeredParticipants []dto.OlympiadParticipantResponse
	duplicateCount := 0

	for _, st := range req.Students {
		studentID := st.StudentID
		studentName := st.StudentName
		gender := st.Gender

		// If studentId is passed, autofill and verify from StudentDetails
		if studentID != "" {
			var dbName, dbGender string
			_ = sqlDB.QueryRow(`SELECT "studentName", gender FROM "StudentDetails" WHERE id = $1 LIMIT 1;`, studentID).Scan(&dbName, &dbGender)
			if dbName != "" {
				studentName = dbName
			}
			if dbGender != "" {
				gender = dbGender
			}
		}

		// Prevent duplicate enrollment for the same test and student
		if testID != "" {
			var existingID string
			if studentID != "" {
				_ = sqlDB.QueryRow(`
					SELECT id FROM "olympiadParticipants" 
					WHERE test_id = $1 AND "studentId" = $2 AND deleted = FALSE LIMIT 1;
				`, testID, studentID).Scan(&existingID)
			}
			if existingID == "" && studentName != "" && schoolCode != "" {
				_ = sqlDB.QueryRow(`
					SELECT id FROM "olympiadParticipants" 
					WHERE test_id = $1 AND LOWER("studentName") = LOWER($2) AND "schoolCode" = $3 AND deleted = FALSE LIMIT 1;
				`, testID, studentName, schoolCode).Scan(&existingID)
			}

			if existingID != "" {
				duplicateCount++
				continue
			}
		}

		partID := uuid.New().String()
		insertQuery := `
			INSERT INTO "olympiadParticipants" (
				id, "studentName", "studentId", "schoolName", "schoolCode", 
				gender, "olympiadTrack", test_id, "createdAt", "createdBy", "updatedAt", deleted
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $9, FALSE
			);
		`

		_, err := sqlDB.Exec(
			insertQuery,
			partID, studentName, studentID, schoolName, schoolCode,
			gender, st.OlympiadTrack, testID, now, creator,
		)

		if err == nil {
			registeredParticipants = append(registeredParticipants, dto.OlympiadParticipantResponse{
				ID:            partID,
				StudentName:   studentName,
				StudentID:     studentID,
				SchoolName:    schoolName,
				SchoolCode:    schoolCode,
				Gender:        gender,
				OlympiadTrack: st.OlympiadTrack,
				TestID:        testID,
				CreatedAt:     now,
				CreatedBy:     creator,
			})
		}
	}

	msg := fmt.Sprintf("Successfully registered %d students for Olympiad.", len(registeredParticipants))
	if duplicateCount > 0 {
		msg += fmt.Sprintf(" (%d students skipped as already enrolled).", duplicateCount)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":      true,
		"message":      msg,
		"total":        len(registeredParticipants),
		"participants": registeredParticipants,
	})
}

// RegisterStudentDirectOlympiad allows an individual student to register directly for an active Olympiad from the student portal
func (h *TechnikHandler) RegisterStudentDirectOlympiad(c *gin.Context) {
	var req dto.DirectStudentOlympiadRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Please specify the Olympiad testId"})
		return
	}

	studentID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "Authentication required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	// Fetch student details
	var studentName, gender, schoolID, grade, email string
	err := sqlDB.QueryRow(`
		SELECT "studentName", COALESCE(gender, 'Male'), COALESCE("schoolId", ''), grade, COALESCE(email, '')
		FROM "StudentDetails" 
		WHERE id = $1 AND "deletedAt" IS NULL LIMIT 1;
	`, studentID).Scan(&studentName, &gender, &schoolID, &grade, &email)

	if err != nil || studentName == "" {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student record not found"})
		return
	}

	var schoolName, schoolCode string
	if schoolID != "" {
		_ = sqlDB.QueryRow(`SELECT "schoolName", "schoolCode" FROM "SchoolDetails" WHERE id = $1 LIMIT 1;`, schoolID).Scan(&schoolName, &schoolCode)
	}
	if schoolName == "" {
		schoolName = "Independent Student Registration"
		schoolCode = "DIRECT"
	}

	// Fetch test details
	var testTitle, track string
	_ = sqlDB.QueryRow(`SELECT title, track FROM olympiad_tests WHERE id = $1 AND is_active = TRUE LIMIT 1;`, req.TestID).Scan(&testTitle, &track)
	if track == "" {
		track = "National Science & Robotics Olympiad"
	}

	// Check if already registered
	var existingID string
	_ = sqlDB.QueryRow(`
		SELECT id FROM "olympiadParticipants" 
		WHERE test_id = $1 AND "studentId" = $2 AND deleted = FALSE LIMIT 1;
	`, req.TestID, studentID).Scan(&existingID)

	if existingID != "" {
		c.JSON(http.StatusConflict, dto.APIError{Success: false, Error: "You are already registered for this Olympiad examination."})
		return
	}

	partID := uuid.New().String()
	now := time.Now()
	creator := email
	if creator == "" {
		creator = "Student Portal"
	}

	insertQuery := `
		INSERT INTO "olympiadParticipants" (
			id, "studentName", "studentId", "schoolName", "schoolCode", 
			gender, "olympiadTrack", test_id, "createdAt", "createdBy", "updatedAt", deleted
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $9, FALSE
		);
	`

	_, err = sqlDB.Exec(
		insertQuery,
		partID, studentName, studentID, schoolName, schoolCode,
		gender, track, req.TestID, now, creator,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to join Olympiad: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":        true,
		"message":        fmt.Sprintf("Successfully registered for %s!", testTitle),
		"participant_id": partID,
		"test_id":        req.TestID,
		"track":          track,
	})
}

// GetOlympiadRegistrations fetches registered olympiad participants from olympiadParticipants table
func (h *TechnikHandler) GetOlympiadRegistrations(c *gin.Context) {
	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	schoolID := c.Query("schoolId")
	userRole, _ := c.Get("userRole")
	if userRole == "school" {
		schoolID = c.GetString("userId")
	} else if schoolID == "" {
		if val, exists := c.Get("userId"); exists {
			schoolID = val.(string)
		}
	}

	var schoolName, schoolCode string
	if schoolID != "" {
		_ = sqlDB.QueryRow(`SELECT "schoolName", "schoolCode" FROM "SchoolDetails" WHERE id = $1 LIMIT 1;`, schoolID).Scan(&schoolName, &schoolCode)
	}

	query := `
		SELECT 
			op.id, op."studentName", COALESCE(op."studentId", ''), 
			COALESCE(op."schoolName", ''), COALESCE(op."schoolCode", ''), 
			op.gender, op."olympiadTrack", COALESCE(op.test_id, ''), 
			COALESCE(t.title, ''), COALESCE(op.olympiadstudentteststatusid, ''),
			COALESCE(s.status, 'NOT_STARTED'), COALESCE(s.score, 0),
			COALESCE(s.certificate_issued, FALSE), COALESCE(s.certificate_url, ''),
			op."createdAt", COALESCE(op."createdBy", ''), op."updatedAt"
		FROM "olympiadParticipants" op
		LEFT JOIN olympiad_tests t ON op.test_id = t.id
		LEFT JOIN olympiadstudentteststatus s ON op.olympiadstudentteststatusid = s.id
		WHERE op.deleted = FALSE
	`
	args := []interface{}{}
	if schoolCode != "" || schoolName != "" {
		query += ` AND (op."schoolCode" = $1 OR op."schoolName" = $2)`
		args = append(args, schoolCode, schoolName)
	}
	query += ` ORDER BY op."createdAt" DESC;`

	rows, err := sqlDB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch registrations: " + err.Error()})
		return
	}
	defer rows.Close()

	var participants []dto.OlympiadParticipantResponse
	for rows.Next() {
		var p dto.OlympiadParticipantResponse
		err := rows.Scan(
			&p.ID, &p.StudentName, &p.StudentID,
			&p.SchoolName, &p.SchoolCode,
			&p.Gender, &p.OlympiadTrack, &p.TestID,
			&p.TestTitle, &p.OlympiadStudentTestStatusID,
			&p.TestStatus, &p.Score,
			&p.CertificateIssued, &p.CertificateURL,
			&p.CreatedAt, &p.CreatedBy, &p.UpdatedAt,
		)
		if err == nil {
			participants = append(participants, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"total":        len(participants),
		"participants": participants,
	})
}
