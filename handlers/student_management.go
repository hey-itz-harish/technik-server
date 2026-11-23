package handlers

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StudentManagementHandler struct {
	cfg *config.Config
}

func NewStudentManagementHandler(cfg *config.Config) *StudentManagementHandler {
	return &StudentManagementHandler{cfg: cfg}
}

// resolveSchoolID extracts the target school ID from context or request
func (h *StudentManagementHandler) resolveSchoolID(c *gin.Context) string {
	if sID, exists := c.Get("schoolId"); exists && sID != "" {
		if idStr, ok := sID.(string); ok && idStr != "" {
			return idStr
		}
	}
	if uRole, exists := c.Get("userRole"); exists && uRole == "school" {
		if uID, ok := c.Get("userId"); ok {
			return uID.(string)
		}
	}
	return c.Query("schoolId")
}

// GetSchoolStudents retrieves all students for a school with search & filter capabilities for the portal and dropdowns
func (h *StudentManagementHandler) GetSchoolStudents(c *gin.Context) {
	_ = database.EnsureConnected()
	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	targetSchoolID := h.resolveSchoolID(c)
	if targetSchoolID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "School identification is required"})
		return
	}

	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	grade := strings.TrimSpace(c.Query("grade"))
	gender := strings.TrimSpace(c.Query("gender"))

	query := `
		SELECT 
			id, "studentName", COALESCE(email, ''), COALESCE(phone, ''), 
			grade, COALESCE(section, ''), COALESCE("rollNo", ''), 
			COALESCE("fatherName", ''), COALESCE("motherName", ''), COALESCE("parentPhone", ''), 
			COALESCE(address, ''), COALESCE("schoolId", ''), 
			COALESCE(gender, ''), COALESCE("academicYear", 2026),
			"createdAt", "updatedAt"
		FROM "StudentDetails"
		WHERE ("schoolId" = $1 OR $1 = '')
		  AND "deletedAt" IS NULL
	`
	args := []interface{}{targetSchoolID}
	argIdx := 2

	if grade != "" && grade != "All" {
		query += ` AND LOWER(grade) = LOWER($` + strconv.Itoa(argIdx) + `)`
		args = append(args, grade)
		argIdx++
	}
	if gender != "" && gender != "All" {
		query += ` AND LOWER(gender) = LOWER($` + strconv.Itoa(argIdx) + `)`
		args = append(args, gender)
		argIdx++
	}
	if search != "" {
		query += ` AND (
			LOWER("studentName") LIKE '%' || $` + strconv.Itoa(argIdx) + ` || '%'
			OR LOWER("rollNo") LIKE '%' || $` + strconv.Itoa(argIdx) + ` || '%'
			OR LOWER("fatherName") LIKE '%' || $` + strconv.Itoa(argIdx) + ` || '%'
			OR LOWER("motherName") LIKE '%' || $` + strconv.Itoa(argIdx) + ` || '%'
			OR LOWER(email) LIKE '%' || $` + strconv.Itoa(argIdx) + ` || '%'
		)`
		args = append(args, search)
		argIdx++
	}

	query += ` ORDER BY "createdAt" DESC;`

	rows, err := sqlDB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to query students: " + err.Error()})
		return
	}
	defer rows.Close()

	var students []dto.StudentResponse
	for rows.Next() {
		var s dto.StudentResponse
		var father, mother, genderVal string
		err := rows.Scan(
			&s.ID, &s.StudentName, &s.Email, &s.Phone,
			&s.Grade, &s.Section, &s.RollNo,
			&father, &mother, &s.ParentPhone,
			&s.CSRFID, &s.SchoolID,
			&genderVal, &s.AcademicYear,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err == nil {
			s.Gender = genderVal
			s.CSRFID = "" // Clear internal field
			s.FatherName = father
			s.MotherName = mother
			if father != "" {
				s.ParentName = father
			} else {
				s.ParentName = mother
			}
			students = append(students, s)
		}
	}

	c.JSON(http.StatusOK, dto.SchoolStudentsResponse{
		Success:  true,
		Message:  "Students retrieved successfully",
		Total:    len(students),
		Students: students,
	})
}

// CreateStudent adds a single student manually
func (h *StudentManagementHandler) CreateStudent(c *gin.Context) {
	var req dto.CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	targetSchoolID := h.resolveSchoolID(c)
	if targetSchoolID == "" {
		targetSchoolID = req.SchoolID
	}
	if targetSchoolID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "School identification is required"})
		return
	}

	id := uuid.New().String()
	now := time.Now()
	year := req.AcademicYear
	if year <= 0 {
		year = now.Year()
	}

	insertQuery := `
		INSERT INTO "StudentDetails" (
			id, "studentName", grade, section, "rollNo", gender, 
			"fatherName", "motherName", "parentPhone", phone, email, address, 
			"schoolId", "academicYear", "isActivated", "isVerified", "createdAt", "updatedAt"
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE, TRUE, $15, $15
		);
	`

	_, err := sqlDB.Exec(
		insertQuery,
		id, req.StudentName, req.Grade, req.Section, req.RollNo, req.Gender,
		req.FatherName, req.MotherName, req.ParentPhone, req.Phone, req.Email, req.Address,
		targetSchoolID, year, now,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create student: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Student created successfully",
		"student": gin.H{
			"id":          id,
			"studentName": req.StudentName,
			"grade":       req.Grade,
			"gender":      req.Gender,
			"rollNo":      req.RollNo,
			"fatherName":  req.FatherName,
			"motherName":  req.MotherName,
			"schoolId":    targetSchoolID,
		},
	})
}

// UpdateStudent updates an existing student's details
func (h *StudentManagementHandler) UpdateStudent(c *gin.Context) {
	studentID := c.Param("id")
	if studentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Student ID is required"})
		return
	}

	var req dto.UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	targetSchoolID := h.resolveSchoolID(c)

	updateQuery := `
		UPDATE "StudentDetails"
		SET 
			"studentName" = COALESCE(NULLIF($1, ''), "studentName"),
			grade = COALESCE(NULLIF($2, ''), grade),
			section = COALESCE(NULLIF($3, ''), section),
			"rollNo" = COALESCE(NULLIF($4, ''), "rollNo"),
			gender = COALESCE(NULLIF($5, ''), gender),
			"fatherName" = COALESCE(NULLIF($6, ''), "fatherName"),
			"motherName" = COALESCE(NULLIF($7, ''), "motherName"),
			"parentPhone" = COALESCE(NULLIF($8, ''), "parentPhone"),
			phone = COALESCE(NULLIF($9, ''), phone),
			email = COALESCE(NULLIF($10, ''), email),
			address = COALESCE(NULLIF($11, ''), address),
			"updatedAt" = CURRENT_TIMESTAMP
		WHERE id = $12 AND ("schoolId" = $13 OR $13 = '') AND "deletedAt" IS NULL;
	`

	res, err := sqlDB.Exec(
		updateQuery,
		req.StudentName, req.Grade, req.Section, req.RollNo, req.Gender,
		req.FatherName, req.MotherName, req.ParentPhone, req.Phone, req.Email, req.Address,
		studentID, targetSchoolID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update student: " + err.Error()})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student updated successfully",
	})
}

// DeleteStudent soft-deletes a single student
func (h *StudentManagementHandler) DeleteStudent(c *gin.Context) {
	studentID := c.Param("id")
	if studentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Student ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	targetSchoolID := h.resolveSchoolID(c)

	res, err := sqlDB.Exec(
		`UPDATE "StudentDetails" SET "deletedAt" = CURRENT_TIMESTAMP WHERE id = $1 AND ("schoolId" = $2 OR $2 = '');`,
		studentID, targetSchoolID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to delete student: " + err.Error()})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student record not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student deleted successfully",
	})
}

// BulkDeleteStudents soft-deletes multiple selected students
func (h *StudentManagementHandler) BulkDeleteStudents(c *gin.Context) {
	var req dto.BulkDeleteStudentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Please provide an array of studentIds"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	targetSchoolID := h.resolveSchoolID(c)
	deletedCount := 0

	for _, id := range req.StudentIDs {
		res, err := sqlDB.Exec(
			`UPDATE "StudentDetails" SET "deletedAt" = CURRENT_TIMESTAMP WHERE id = $1 AND ("schoolId" = $2 OR $2 = '') AND "deletedAt" IS NULL;`,
			id, targetSchoolID,
		)
		if err == nil {
			if count, _ := res.RowsAffected(); count > 0 {
				deletedCount++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Selected students deleted successfully",
		"deleted_count": deletedCount,
	})
}

// UploadStudentsExcel handles CSV or Excel bulk student import
func (h *StudentManagementHandler) UploadStudentsExcel(c *gin.Context) {
	targetSchoolID := h.resolveSchoolID(c)
	if targetSchoolID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "School identification is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	file, err := c.FormFile("file")
	var records [][]string

	if err == nil {
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Failed to open uploaded file: " + err.Error()})
			return
		}
		defer src.Close()

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(src)

		// Parse as CSV
		reader := csv.NewReader(bytes.NewReader(buf.Bytes()))
		reader.LazyQuotes = true
		reader.FieldsPerRecord = -1
		parsed, err := reader.ReadAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Unable to parse CSV/Excel data. Please ensure it is a valid format: " + err.Error()})
			return
		}
		records = parsed
	} else {
		// Fallback to JSON payload
		var jsonReq struct {
			Students []dto.StudentExcelRow `json:"students"`
		}
		if err := c.ShouldBindJSON(&jsonReq); err != nil || len(jsonReq.Students) == 0 {
			c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Upload a CSV/Excel file under 'file' key or provide 'students' JSON array."})
			return
		}

		imported := 0
		for _, s := range jsonReq.Students {
			if s.StudentName == "" || s.Grade == "" {
				continue
			}
			id := uuid.New().String()
			now := time.Now()
			year := s.AcademicYear
			if year <= 0 {
				year = now.Year()
			}
			_, err := sqlDB.Exec(`
				INSERT INTO "StudentDetails" (
					id, "studentName", grade, section, "rollNo", gender, 
					"fatherName", "motherName", "parentPhone", phone, email, address, 
					"schoolId", "academicYear", "isActivated", "isVerified", "createdAt", "updatedAt"
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE, TRUE, $15, $15)
				ON CONFLICT DO NOTHING;
			`, id, s.StudentName, s.Grade, s.Section, s.RollNo, s.Gender,
				s.FatherName, s.MotherName, s.ParentPhone, s.Phone, s.Email, s.Address,
				targetSchoolID, year, now)
			if err == nil {
				imported++
			}
		}

		c.JSON(http.StatusOK, dto.StudentExcelImportResponse{
			Success:       true,
			Message:       "Students imported successfully",
			TotalRows:     len(jsonReq.Students),
			ImportedCount: imported,
		})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "File must contain a header row and at least one student data row."})
		return
	}

	headers := records[0]
	headerMap := make(map[string]int)
	for idx, hName := range headers {
		cleaned := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(hName, "_", "")))
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		headerMap[cleaned] = idx
	}

	getVal := func(row []string, keys ...string) string {
		for _, k := range keys {
			if idx, ok := headerMap[k]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
		}
		return ""
	}

	imported := 0
	updated := 0
	failed := 0
	var errorLogs []string
	now := time.Now()

	for rowIdx := 1; rowIdx < len(records); rowIdx++ {
		row := records[rowIdx]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		name := getVal(row, "studentname", "name", "student")
		grade := getVal(row, "grade", "class", "standard")
		if name == "" || grade == "" {
			failed++
			errorLogs = append(errorLogs, "Row "+strconv.Itoa(rowIdx+1)+": Missing student name or grade")
			continue
		}

		section := getVal(row, "section", "sec")
		rollNo := getVal(row, "rollno", "rollnumber", "roll")
		gender := getVal(row, "gender", "sex")
		fatherName := getVal(row, "fathername", "father", "father'sname")
		motherName := getVal(row, "mothername", "mother", "mother'sname")
		parentPhone := getVal(row, "parentphone", "parentmobile", "guardianphone")
		phone := getVal(row, "phone", "studentphone", "mobile")
		email := getVal(row, "email", "emailaddress")
		address := getVal(row, "address", "city")
		yearStr := getVal(row, "academicyear", "year")
		year, _ := strconv.Atoi(yearStr)
		if year <= 0 {
			year = now.Year()
		}

		// Check if student with same rollNo or email exists for this school
		var existingID string
		if rollNo != "" {
			_ = sqlDB.QueryRow(
				`SELECT id FROM "StudentDetails" WHERE "schoolId" = $1 AND "rollNo" = $2 AND "deletedAt" IS NULL LIMIT 1;`,
				targetSchoolID, rollNo,
			).Scan(&existingID)
		}
		if existingID == "" && email != "" {
			_ = sqlDB.QueryRow(
				`SELECT id FROM "StudentDetails" WHERE "schoolId" = $1 AND email = $2 AND "deletedAt" IS NULL LIMIT 1;`,
				targetSchoolID, email,
			).Scan(&existingID)
		}

		if existingID != "" {
			// Update existing student
			_, err := sqlDB.Exec(`
				UPDATE "StudentDetails"
				SET "studentName" = $1, grade = $2, section = $3, gender = $4,
				    "fatherName" = $5, "motherName" = $6, "parentPhone" = $7,
				    phone = $8, address = $9, "academicYear" = $10, "updatedAt" = $11
				WHERE id = $12;
			`, name, grade, section, gender, fatherName, motherName, parentPhone, phone, address, year, now, existingID)
			if err == nil {
				updated++
			} else {
				failed++
				errorLogs = append(errorLogs, "Row "+strconv.Itoa(rowIdx+1)+": Update error: "+err.Error())
			}
		} else {
			// Insert new student
			newID := uuid.New().String()
			_, err := sqlDB.Exec(`
				INSERT INTO "StudentDetails" (
					id, "studentName", grade, section, "rollNo", gender, 
					"fatherName", "motherName", "parentPhone", phone, email, address, 
					"schoolId", "academicYear", "isActivated", "isVerified", "createdAt", "updatedAt"
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE, TRUE, $15, $15);
			`, newID, name, grade, section, rollNo, gender, fatherName, motherName, parentPhone, phone, email, address, targetSchoolID, year, now)
			if err == nil {
				imported++
			} else {
				failed++
				errorLogs = append(errorLogs, "Row "+strconv.Itoa(rowIdx+1)+": Insert error: "+err.Error())
			}
		}
	}

	c.JSON(http.StatusOK, dto.StudentExcelImportResponse{
		Success:       true,
		Message:       "Excel/CSV student processing complete",
		TotalRows:     len(records) - 1,
		ImportedCount: imported,
		UpdatedCount:  updated,
		FailedCount:   failed,
		Errors:        errorLogs,
	})
}
