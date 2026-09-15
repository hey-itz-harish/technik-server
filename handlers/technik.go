package handlers

import (
	"context"
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
)

type TechnikHandler struct {
	cfg *config.Config
}

func NewTechnikHandler(cfg *config.Config) *TechnikHandler {
	return &TechnikHandler{cfg: cfg}
}

// Helper to determine if a class is Junior (Grades 3-5) or Senior (Grades 6-8)
func isJuniorLevel(classStr, classCat string) bool {
	combined := strings.ToLower(classStr + " " + classCat)
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

// UploadDocument handles multipart file upload for nominations / certificates
func (h *TechnikHandler) UploadDocument(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "No file uploaded or invalid form key 'file'"})
		return
	}

	// Create directory if not exists
	uploadDir := "./uploads/nominations"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
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

		targetSchoolID := batchReq.SchoolID
		if targetSchoolID == "" {
			targetSchoolID = sessionSchoolID
		}

		// Fetch existing nominations for this school and year to check quota
		var existingNoms []db.TechnikPrideNominationModel
		if targetSchoolID != "" {
			existingNoms, _ = database.Client.TechnikPrideNomination.FindMany(
				db.TechnikPrideNomination.AcademicYear.Equals(year),
				db.TechnikPrideNomination.SchoolID.Equals(targetSchoolID),
			).Exec(ctx)
		}

		var existingJrCount, existingSrCount int
		for _, ex := range existingNoms {
			cat, _ := ex.ClassCategory()
			if isJuniorLevel(ex.Class, cat) {
				existingJrCount++
			} else {
				existingSrCount++
			}
		}

		// Count incoming
		var incomingJrCount, incomingSrCount int
		for _, nom := range batchReq.Nominations {
			if isJuniorLevel(nom.Class, nom.ClassCategory) {
				incomingJrCount++
			} else {
				incomingSrCount++
			}
		}

		// Enforce Quota: Max 3 Junior & Max 3 Senior per year (Total 6)
		if existingJrCount+incomingJrCount > 3 {
			c.JSON(http.StatusBadRequest, dto.APIError{
				Success: false,
				Error: fmt.Sprintf("Annual Quota Limit Exceeded: Junior Level (Grades 3-5) allows a maximum of 3 nominations per year. You have %d registered and attempted to add %d more.", existingJrCount, incomingJrCount),
			})
			return
		}

		if existingSrCount+incomingSrCount > 3 {
			c.JSON(http.StatusBadRequest, dto.APIError{
				Success: false,
				Error: fmt.Sprintf("Annual Quota Limit Exceeded: Senior Level (Grades 6-8) allows a maximum of 3 nominations per year. You have %d registered and attempted to add %d more.", existingSrCount, incomingSrCount),
			})
			return
		}

		var createdResponses []dto.TechnikPrideNominationResponse
		for _, nom := range batchReq.Nominations {
			schoolID := nom.SchoolID
			if schoolID == "" {
				schoolID = targetSchoolID
			}

			normCategory := "Grade 6 to 8 (Senior Level)"
			if isJuniorLevel(nom.Class, nom.ClassCategory) {
				normCategory = "Grade 3 to 5 (Jr Level)"
			}
			if nom.ClassCategory != "" {
				normCategory = nom.ClassCategory
			}

			params := []db.TechnikPrideNominationSetParam{
				db.TechnikPrideNomination.AcademicYear.Set(year),
				db.TechnikPrideNomination.ClassCategory.Set(normCategory),
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
				clsCat, _ := res.ClassCategory()
				supDoc, _ := res.SupportingDocument()
				nomStat, _ := res.NominationStatus()
				admStat, _ := res.AdminStatus()

				createdResponses = append(createdResponses, dto.TechnikPrideNominationResponse{
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

				// Also add student into StudentDetails schema
				var stParams []db.StudentDetailsSetParam
				stParams = append(stParams,
					db.StudentDetails.Gender.Set(nom.Gender),
					db.StudentDetails.Category.Set("Technik Pride Award"),
					db.StudentDetails.ClassCategory.Set(normCategory),
					db.StudentDetails.Status.Set("Nomination Received"),
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

	targetSchoolID := singleReq.SchoolID
	if targetSchoolID == "" {
		targetSchoolID = sessionSchoolID
	}

	// Check existing nominations for this school and year
	var existingNoms []db.TechnikPrideNominationModel
	if targetSchoolID != "" {
		existingNoms, _ = database.Client.TechnikPrideNomination.FindMany(
			db.TechnikPrideNomination.AcademicYear.Equals(year),
			db.TechnikPrideNomination.SchoolID.Equals(targetSchoolID),
		).Exec(ctx)
	}

	var existingJrCount, existingSrCount int
	for _, ex := range existingNoms {
		cat, _ := ex.ClassCategory()
		if isJuniorLevel(ex.Class, cat) {
			existingJrCount++
		} else {
			existingSrCount++
		}
	}

	isJr := isJuniorLevel(singleReq.Class, singleReq.ClassCategory)
	if isJr && existingJrCount >= 3 {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   fmt.Sprintf("Annual Quota Limit Exceeded: Junior Level (Grades 3-5) allows a maximum of 3 nominations per year (currently %d registered).", existingJrCount),
		})
		return
	}
	if !isJr && existingSrCount >= 3 {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   fmt.Sprintf("Annual Quota Limit Exceeded: Senior Level (Grades 6-8) allows a maximum of 3 nominations per year (currently %d registered).", existingSrCount),
		})
		return
	}

	normCategory := "Grade 6 to 8 (Senior Level)"
	if isJr {
		normCategory = "Grade 3 to 5 (Jr Level)"
	}
	if singleReq.ClassCategory != "" {
		normCategory = singleReq.ClassCategory
	}

	params := []db.TechnikPrideNominationSetParam{
		db.TechnikPrideNomination.AcademicYear.Set(year),
		db.TechnikPrideNomination.ClassCategory.Set(normCategory),
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
	clsCat, _ := res.ClassCategory()
	supDoc, _ := res.SupportingDocument()
	nomStat, _ := res.NominationStatus()
	admStat, _ := res.AdminStatus()

	// Also sync into StudentDetails schema
	var stParams []db.StudentDetailsSetParam
	stParams = append(stParams,
		db.StudentDetails.Gender.Set(singleReq.Gender),
		db.StudentDetails.Category.Set("Technik Pride Award"),
		db.StudentDetails.ClassCategory.Set(normCategory),
		db.StudentDetails.Status.Set("Nomination Received"),
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

	if schoolID == "" {
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
	schoolID := c.Query("schoolId")
	if schoolID == "" {
		if val, exists := c.Get("userId"); exists {
			schoolID = val.(string)
		}
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
				if isJuniorLevel(nom.Class, "") {
					clsCat = "Grade 3 to 5 (Jr Level)"
				} else {
					clsCat = "Grade 6 to 8 (Senior Level)"
				}
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
			if clsCat != "" && !strings.Contains(displayGrade, "(") {
				if strings.Contains(clsCat, "Jr") {
					displayGrade = fmt.Sprintf("%s (Jr Level)", nom.Class)
				} else if strings.Contains(clsCat, "Senior") || strings.Contains(clsCat, "Sr") {
					displayGrade = fmt.Sprintf("%s (Sr Level)", nom.Class)
				}
			}

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

	// 2. Fetch Olympiad Registrations and Olympiad Students
	var olymRegistrations []db.OlympiadRegistrationModel
	if schoolID != "" {
		olymRegistrations, err = database.Client.OlympiadRegistration.FindMany(
			db.OlympiadRegistration.SchoolID.Equals(schoolID),
		).With(
			db.OlympiadRegistration.Students.Fetch(),
		).OrderBy(
			db.OlympiadRegistration.CreatedAt.Order(db.SortOrderDesc),
		).Exec(ctx)
	} else {
		olymRegistrations, err = database.Client.OlympiadRegistration.FindMany().With(
			db.OlympiadRegistration.Students.Fetch(),
		).OrderBy(
			db.OlympiadRegistration.CreatedAt.Order(db.SortOrderDesc),
		).Exec(ctx)
	}

	if err == nil {
		for _, reg := range olymRegistrations {
			schID, _ := reg.SchoolID()
			for _, st := range reg.Students() {
				clsCat, _ := st.ClassCategory()
				if clsCat == "" {
					if isJuniorLevel(st.Class, "") {
						clsCat = "Grade 3 to 5 (Jr Level)"
					} else {
						clsCat = "Grade 6 to 8 (Senior Level)"
					}
				}

				category := st.OlympiadTrack
				if category == "" {
					category = "Robotics Olympiad"
				}

				displayGrade := st.Class
				if clsCat != "" && !strings.Contains(displayGrade, "(") {
					if strings.Contains(clsCat, "Jr") {
						displayGrade = fmt.Sprintf("%s (Jr Level)", st.Class)
					} else if strings.Contains(clsCat, "Senior") || strings.Contains(clsCat, "Sr") {
						displayGrade = fmt.Sprintf("%s (Sr Level)", st.Class)
					}
				}

				combined = append(combined, dto.StudentResponse{
					ID:            st.ID,
					StudentName:   st.StudentName,
					Grade:         displayGrade,
					ClassCategory: clsCat,
					Category:      category,
					Gender:        st.Gender,
					Status:        "Registered & Verified",
					AcademicYear:  reg.AcademicYear,
					SchoolID:      schID,
					CreatedAt:     st.CreatedAt,
					UpdatedAt:     st.UpdatedAt,
				})
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
			category, _ := st.Category()
			if category == "" {
				category = "Robotics Olympiad"
			}
			clsCat, _ := st.ClassCategory()
			status, _ := st.Status()
			if status == "" {
				status = "Registered & Verified"
			}
			acadYear, ok := st.AcademicYear()
			if !ok || acadYear == 0 {
				acadYear = st.CreatedAt.Year()
			}

			displayGrade := st.Grade
			if clsCat != "" && !strings.Contains(displayGrade, "(") {
				if strings.Contains(clsCat, "Jr") {
					displayGrade = fmt.Sprintf("%s (Jr Level)", st.Grade)
				} else if strings.Contains(clsCat, "Senior") || strings.Contains(clsCat, "Sr") {
					displayGrade = fmt.Sprintf("%s (Sr Level)", st.Grade)
				}
			}

			combined = append(combined, dto.StudentResponse{
				ID:            st.ID,
				StudentName:   st.StudentName,
				Grade:         displayGrade,
				ClassCategory: clsCat,
				Category:      category,
				Gender:        gender,
				Status:        status,
				AcademicYear:  acadYear,
				SchoolID:      schID,
				CreatedAt:     st.CreatedAt,
				UpdatedAt:     st.UpdatedAt,
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
				if !strings.Contains(st.Grade, "Jr") && !isJuniorLevel(st.Grade, st.ClassCategory) {
					continue
				}
			} else if gradeLevel == "Sr Level" || gradeLevel == "Senior Level" {
				if !strings.Contains(st.Grade, "Sr") && isJuniorLevel(st.Grade, st.ClassCategory) {
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

// RegisterOlympiad handles student registration in array for Olympiad tracks
// and also adds registered students into StudentDetails schema with category "Olympiad - {track}".
func (h *TechnikHandler) RegisterOlympiad(c *gin.Context) {
	var req dto.OlympiadRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	currentYear := time.Now().Year()
	year := req.AcademicYear
	if year <= 0 {
		year = currentYear
	}

	targetSchoolID := req.SchoolID
	if targetSchoolID == "" {
		if val, exists := c.Get("userId"); exists {
			targetSchoolID = val.(string)
		}
	}

	// Create Olympiad Registration Batch Header
	var regParams []db.OlympiadRegistrationSetParam
	regParams = append(regParams, db.OlympiadRegistration.AcademicYear.Set(year))
	regParams = append(regParams, db.OlympiadRegistration.TotalStudents.Set(len(req.Students)))

	if targetSchoolID != "" {
		regParams = append(regParams, db.OlympiadRegistration.School.Link(
			db.SchoolDetails.ID.Equals(targetSchoolID),
		))
	}

	registration, err := database.Client.OlympiadRegistration.CreateOne(
		regParams...,
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create Olympiad registration: " + err.Error()})
		return
	}

	// Create student entries in array and sync into StudentDetails
	var studentResponses []dto.OlympiadStudentResponse
	for _, st := range req.Students {
		normCategory := "Grade 6 to 8 (Senior Level)"
		if isJuniorLevel(st.Class, st.ClassCategory) {
			normCategory = "Grade 3 to 5 (Jr Level)"
		}
		if st.ClassCategory != "" {
			normCategory = st.ClassCategory
		}

		stParams := []db.OlympiadStudentSetParam{
			db.OlympiadStudent.ClassCategory.Set(normCategory),
		}

		stRes, err := database.Client.OlympiadStudent.CreateOne(
			db.OlympiadStudent.OlympiadRegistration.Link(
				db.OlympiadRegistration.ID.Equals(registration.ID),
			),
			db.OlympiadStudent.StudentName.Set(st.StudentName),
			db.OlympiadStudent.Class.Set(st.Class),
			db.OlympiadStudent.Gender.Set(st.Gender),
			db.OlympiadStudent.OlympiadTrack.Set(st.OlympiadTrack),
			stParams...,
		).Exec(ctx)

		if err == nil && stRes != nil {
			cat, _ := stRes.ClassCategory()
			studentResponses = append(studentResponses, dto.OlympiadStudentResponse{
				ID:            stRes.ID,
				StudentName:   stRes.StudentName,
				Class:         stRes.Class,
				ClassCategory: cat,
				Gender:        stRes.Gender,
				OlympiadTrack: stRes.OlympiadTrack,
				CreatedAt:     stRes.CreatedAt,
			})

			// Add to StudentDetails schema
			var sdParams []db.StudentDetailsSetParam
			sdParams = append(sdParams,
				db.StudentDetails.Gender.Set(st.Gender),
				db.StudentDetails.Category.Set(st.OlympiadTrack),
				db.StudentDetails.ClassCategory.Set(normCategory),
				db.StudentDetails.Status.Set("Registered & Verified"),
				db.StudentDetails.AcademicYear.Set(year),
			)
			if targetSchoolID != "" {
				sdParams = append(sdParams, db.StudentDetails.School.Link(db.SchoolDetails.ID.Equals(targetSchoolID)))
			}

			_, _ = database.Client.StudentDetails.CreateOne(
				db.StudentDetails.StudentName.Set(st.StudentName),
				db.StudentDetails.Grade.Set(st.Class),
				sdParams...,
			).Exec(ctx)
		}
	}

	schID, _ := registration.SchoolID()
	c.JSON(http.StatusCreated, dto.OlympiadBatchResponse{
		Success: true,
		Message: "Olympiad student registrations submitted successfully",
		Registration: dto.OlympiadRegistrationResponse{
			ID:            registration.ID,
			SchoolID:      schID,
			AcademicYear:  registration.AcademicYear,
			TotalStudents: len(studentResponses),
			Students:      studentResponses,
			CreatedAt:     registration.CreatedAt,
		},
	})
}

// GetOlympiadRegistrations fetches registered olympiad students/batches
func (h *TechnikHandler) GetOlympiadRegistrations(c *gin.Context) {
	ctx := context.Background()
	schoolID := c.Query("schoolId")
	yearStr := c.Query("year")

	if schoolID == "" {
		if val, exists := c.Get("userId"); exists {
			schoolID = val.(string)
		}
	}

	year := time.Now().Year()
	if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
		year = y
	}

	var registrations []db.OlympiadRegistrationModel
	var err error

	if schoolID != "" {
		registrations, err = database.Client.OlympiadRegistration.FindMany(
			db.OlympiadRegistration.AcademicYear.Equals(year),
			db.OlympiadRegistration.SchoolID.Equals(schoolID),
		).With(
			db.OlympiadRegistration.Students.Fetch(),
		).Exec(ctx)
	} else {
		registrations, err = database.Client.OlympiadRegistration.FindMany(
			db.OlympiadRegistration.AcademicYear.Equals(year),
		).With(
			db.OlympiadRegistration.Students.Fetch(),
		).Exec(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch registrations: " + err.Error()})
		return
	}

	var results []dto.OlympiadRegistrationResponse
	for _, reg := range registrations {
		schID, _ := reg.SchoolID()
		var stResps []dto.OlympiadStudentResponse
		for _, st := range reg.Students() {
			cat, _ := st.ClassCategory()
			stResps = append(stResps, dto.OlympiadStudentResponse{
				ID:            st.ID,
				StudentName:   st.StudentName,
				Class:         st.Class,
				ClassCategory: cat,
				Gender:        st.Gender,
				OlympiadTrack: st.OlympiadTrack,
				CreatedAt:     st.CreatedAt,
			})
		}

		results = append(results, dto.OlympiadRegistrationResponse{
			ID:            reg.ID,
			SchoolID:      schID,
			AcademicYear:  reg.AcademicYear,
			TotalStudents: len(stResps),
			Students:      stResps,
			CreatedAt:     reg.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Olympiad registrations retrieved successfully",
		"total":         len(results),
		"registrations": results,
	})
}
