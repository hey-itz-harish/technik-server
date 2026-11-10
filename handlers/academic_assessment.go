package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AcademicAssessmentHandler struct {
	cfg *config.Config
}

func NewAcademicAssessmentHandler(cfg *config.Config) *AcademicAssessmentHandler {
	return &AcademicAssessmentHandler{cfg: cfg}
}

// resolveSchoolInfo extracts school ID and Name from context, DB, or query
func (h *AcademicAssessmentHandler) resolveSchoolInfo(c *gin.Context) (string, string, string) {
	_ = database.EnsureConnected()
	var schoolID, schoolName, userEmail string

	if val, ok := c.Get("userId"); ok && val != nil {
		schoolID = val.(string)
	}
	if val, ok := c.Get("userEmail"); ok && val != nil {
		userEmail = val.(string)
	}

	// If schoolId passed explicitly as query or header fallback
	if qID := c.Query("schoolId"); qID != "" && schoolID == "" {
		schoolID = qID
	}

	// Retrieve School Name from DB if schoolID exists
	if schoolID != "" {
		sch, err := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.ID.Equals(schoolID),
		).Exec(c.Request.Context())
		if err == nil && sch != nil {
			schoolName = sch.SchoolName
		}
	}

	if schoolName == "" && userEmail != "" {
		sch, err := database.Client.SchoolDetails.FindUnique(
			db.SchoolDetails.Email.Equals(userEmail),
		).Exec(c.Request.Context())
		if err == nil && sch != nil {
			schoolID = sch.ID
			schoolName = sch.SchoolName
		}
	}

	if schoolName == "" {
		schoolName = "Technik Partner School"
	}

	return schoolID, schoolName, userEmail
}

// CreateAcademicAssessment handles uploading question paper doc, scraping sessions via OpenAI/local, and mapping students
func (h *AcademicAssessmentHandler) CreateAcademicAssessment(c *gin.Context) {
	schoolID, schoolName, userEmail := h.resolveSchoolInfo(c)
	if schoolID == "" {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "School authentication required"})
		return
	}

	examName := strings.TrimSpace(c.PostForm("exam_name"))
	targetClass := strings.TrimSpace(c.PostForm("class"))
	examDate := strings.TrimSpace(c.PostForm("exam_date"))

	if examName == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Exam Name is required"})
		return
	}
	if targetClass == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Class is required"})
		return
	}
	if examDate == "" {
		examDate = time.Now().Format("2006-01-02")
	}

	// Retrieve uploaded document file
	file, header, err := c.Request.FormFile("question_paper")
	if err != nil {
		file, header, err = c.Request.FormFile("document")
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Question paper document file is required"})
		return
	}
	defer file.Close()

	// Strict file type validation: ONLY doc and docx are allowed
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".doc" && ext != ".docx" {
		c.JSON(http.StatusBadRequest, dto.APIError{
			Success: false,
			Error:   "Invalid file type. Only Word documents (.doc, .docx) are allowed for question papers.",
		})
		return
	}

	// Maximum upload size: 25 MB
	if header.Size > 25*1024*1024 {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "File size exceeds 25MB limit"})
		return
	}

	// Save file locally to ./uploads/academic_assessment/
	uploadDir := "./uploads/academic_assessment"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create upload directory"})
		return
	}

	uniqueFilename := fmt.Sprintf("exam_%s_%d%s", uuid.New().String()[:8], time.Now().Unix(), ext)
	savedFilePath := filepath.Join(uploadDir, uniqueFilename)

	dst, err := os.Create(savedFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to save question paper file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to write question paper file"})
		return
	}

	// Step 1: Extract Text from Document
	docText, err := ExtractTextFromDocument(savedFilePath)
	if err != nil {
		log.Printf("[Document Parser] Warning extracting text from %s: %v", savedFilePath, err)
		docText = fmt.Sprintf("Examination: %s\nClass: %s\nDate: %s\nSection I: General Questions (10 Marks)", examName, targetClass, examDate)
	}

	// Step 2: Scrape Question Paper with OpenAI (with intelligent fallback)
	scraped, err := ScrapeQuestionPaperWithOpenAI(docText, h.cfg.OpenAIKey)
	if err != nil || scraped == nil {
		scraped = ParseQuestionPaperLocally(docText)
	}

	// Step 3: Insert into academic_assessments Table
	assessmentID := "ASM-" + strings.ToUpper(uuid.New().String()[:12])
	questionsJSON, err := json.Marshal(scraped.Sessions)
	if err != nil {
		questionsJSON = []byte("[]")
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	insertQuery := `
		INSERT INTO academic_assessments (
			id, school_name, school_id, exam_name, exam_question_paper_document_path,
			class, exam_date, number_of_questions, total_mark, questions,
			created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);
	`

	now := time.Now()
	webDocPath := "/uploads/academic_assessment/" + uniqueFilename
	_, err = sqlDB.Exec(
		insertQuery,
		assessmentID, schoolName, schoolID, examName, webDocPath,
		targetClass, examDate, scraped.NumberOfQuestions, scraped.TotalMark, string(questionsJSON),
		now, userEmail, now, userEmail,
	)
	if err != nil {
		log.Printf("Error inserting academic assessment: %v", err)
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create assessment in database: " + err.Error()})
		return
	}

	// Step 4: Map Students for this School & Class, and Generate Answer Sheet Grids
	mappedCount := h.mapStudentsToAssessment(sqlDB, assessmentID, schoolID, schoolName, examName, examDate, targetClass, scraped)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": fmt.Sprintf("Academic Assessment created successfully with %d questions. Mapped %d students.", scraped.NumberOfQuestions, mappedCount),
		"assessment": dto.AcademicAssessmentDTO{
			ID:                            assessmentID,
			SchoolName:                    schoolName,
			SchoolID:                      schoolID,
			ExamName:                      examName,
			ExamQuestionPaperDocumentPath: webDocPath,
			Class:                         targetClass,
			ExamDate:                      examDate,
			NumberOfQuestions:             scraped.NumberOfQuestions,
			TotalMark:                     scraped.TotalMark,
			Questions:                     scraped.Sessions,
			TotalStudents:                 mappedCount,
			CreatedAt:                     now.Format(time.RFC3339),
			CreatedBy:                     userEmail,
			UpdatedAt:                     now.Format(time.RFC3339),
			UpdatedBy:                     userEmail,
		},
		"scraped_data": scraped,
	})
}

// mapStudentsToAssessment generates the 6-property answer sheet grid for all matching students in the school
func (h *AcademicAssessmentHandler) mapStudentsToAssessment(
	sqlDB *sql.DB,
	assessmentID, schoolID, schoolName, examName, examDate, targetClass string,
	scraped *dto.ScrapedAssessmentData,
) int {
	// Construct the template grid with the 6 properties (plus metadata)
	var templateGrid []dto.GridRowDTO
	for _, sess := range scraped.Sessions {
		for _, q := range sess.Questions {
			templateGrid = append(templateGrid, dto.GridRowDTO{
				SessionNo:             sess.SessionNo,
				QuestionNumber:        q.QuestionNumber,
				SessionAllocatedMark:  sess.SessionAllocatedMark,
				QuestionAllocatedMark: q.QuestionAllocatedMark,
				AnswerMark:            0.0, // Initial un-evaluated score
				ExamName:              examName,
				ExamDate:              examDate,
			})
		}
	}

	gridJSON, _ := json.Marshal(templateGrid)

	// Step A: Find matching students from StudentDetails, TechnikPrideNomination, OlympiadStudent
	type simpleStudent struct {
		ID   string
		Name string
	}
	studentMap := make(map[string]string) // ID -> Name

	normTarget := strings.ToLower(strings.TrimSpace(targetClass))

	// 1. Direct StudentDetails
	rowsSD, err := sqlDB.Query(
		`SELECT id, "studentName", grade FROM "StudentDetails" WHERE "schoolId" = $1 AND "deletedAt" IS NULL;`,
		schoolID,
	)
	if err == nil {
		defer rowsSD.Close()
		for rowsSD.Next() {
			var id, name, grade string
			if err := rowsSD.Scan(&id, &name, &grade); err == nil {
				if classMatches(grade, normTarget) {
					studentMap[id] = name
				}
			}
		}
	}

	// 2. TechnikPrideNomination
	rowsPride, err := sqlDB.Query(
		`SELECT id, "studentName", class FROM "TechnikPrideNomination" WHERE "schoolId" = $1;`,
		schoolID,
	)
	if err == nil {
		defer rowsPride.Close()
		for rowsPride.Next() {
			var id, name, class string
			if err := rowsPride.Scan(&id, &name, &class); err == nil {
				if classMatches(class, normTarget) {
					studentMap[id] = name
				}
			}
		}
	}

	// 3. OlympiadStudent via OlympiadRegistration
	rowsOlym, err := sqlDB.Query(
		`SELECT os.id, os."studentName", os.class 
		 FROM "OlympiadStudent" os 
		 JOIN "OlympiadRegistration" o ON os."olympiadRegistrationId" = o.id 
		 WHERE o."schoolId" = $1;`,
		schoolID,
	)
	if err == nil {
		defer rowsOlym.Close()
		for rowsOlym.Next() {
			var id, name, class string
			if err := rowsOlym.Scan(&id, &name, &class); err == nil {
				if classMatches(class, normTarget) {
					studentMap[id] = name
				}
			}
		}
	}

	// Step B: Insert student evaluation record for each student
	mappedCount := 0
	now := time.Now()

	insertEvalQuery := `
		INSERT INTO student_academic_evaluations (
			id, assessment_id, student_id, student_name, school_id, school_name,
			class, exam_name, exam_date, grid_data, total_marks, obtained_marks,
			percentage, status, created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18);
	`

	for sID, sName := range studentMap {
		evalID := "EVAL-" + strings.ToUpper(uuid.New().String()[:12])
		_, err := sqlDB.Exec(
			insertEvalQuery,
			evalID, assessmentID, sID, sName, schoolID, schoolName,
			targetClass, examName, examDate, string(gridJSON), scraped.TotalMark, 0.0,
			0.0, "Pending", now, "School System", now, "School System",
		)
		if err == nil {
			mappedCount++
			// Also sync initial assessment grid to StudentDetails table
			_, _ = sqlDB.Exec(
				`UPDATE "StudentDetails" 
				 SET "assessment_grid" = $1, "assessment_status" = 'Pending' 
				 WHERE id = $2;`,
				string(gridJSON), sID,
			)
		}
	}

	return mappedCount
}

func classMatches(actual, target string) bool {
	act := strings.ToLower(strings.TrimSpace(actual))
	tgt := strings.ToLower(strings.TrimSpace(target))

	if act == tgt || strings.Contains(act, tgt) || strings.Contains(tgt, act) {
		return true
	}

	// Normalize digits e.g. "5" in "Class 5" vs "Grade 5"
	actNum := extractDigits(act)
	tgtNum := extractDigits(tgt)
	if actNum != "" && tgtNum != "" && actNum == tgtNum {
		return true
	}

	return false
}

func extractDigits(s string) string {
	var digits strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
		}
	}
	return digits.String()
}

// GetAcademicAssessments lists all assessments for the school with student & grading summary statistics
func (h *AcademicAssessmentHandler) GetAcademicAssessments(c *gin.Context) {
	schoolID, _, _ := h.resolveSchoolInfo(c)
	if schoolID == "" {
		c.JSON(http.StatusUnauthorized, dto.APIError{Success: false, Error: "School authentication required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	query := `
		SELECT 
			a.id, a.school_name, a.school_id, a.exam_name, a.exam_question_paper_document_path,
			a.class, a.exam_date, a.number_of_questions, a.total_mark, a.questions,
			a.created_at, a.created_by, a.updated_at, a.updated_by,
			COUNT(e.id) AS total_students,
			COUNT(CASE WHEN e.status = 'Evaluated' THEN 1 END) AS evaluated_students,
			COUNT(CASE WHEN e.status = 'Promoted' THEN 1 END) AS promoted_students,
			COUNT(CASE WHEN e.status = 'Debarred' THEN 1 END) AS debarred_students
		FROM academic_assessments a
		LEFT JOIN student_academic_evaluations e ON a.id = e.assessment_id AND e.deleted_at IS NULL
		WHERE a.school_id = $1 AND a.deleted_at IS NULL
		GROUP BY a.id
		ORDER BY a.created_at DESC;
	`

	rows, err := sqlDB.Query(query, schoolID)
	if err != nil {
		log.Printf("Error querying academic assessments: %v", err)
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch assessments: " + err.Error()})
		return
	}
	defer rows.Close()

	var assessments []dto.AcademicAssessmentDTO
	for rows.Next() {
		var asm dto.AcademicAssessmentDTO
		var questionsRaw string
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&asm.ID, &asm.SchoolName, &asm.SchoolID, &asm.ExamName, &asm.ExamQuestionPaperDocumentPath,
			&asm.Class, &asm.ExamDate, &asm.NumberOfQuestions, &asm.TotalMark, &questionsRaw,
			&createdAt, &asm.CreatedBy, &updatedAt, &asm.UpdatedBy,
			&asm.TotalStudents, &asm.EvaluatedStudents, &asm.PromotedStudents, &asm.DebarredStudents,
		)
		if err != nil {
			continue
		}

		_ = json.Unmarshal([]byte(questionsRaw), &asm.Questions)
		asm.CreatedAt = createdAt.Format(time.RFC3339)
		asm.UpdatedAt = updatedAt.Format(time.RFC3339)

		assessments = append(assessments, asm)
	}

	if assessments == nil {
		assessments = []dto.AcademicAssessmentDTO{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"assessments": assessments,
	})
}

// GetAcademicAssessmentByID returns a specific assessment with its full questions list
func (h *AcademicAssessmentHandler) GetAcademicAssessmentByID(c *gin.Context) {
	assessmentID := c.Param("id")
	if assessmentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Assessment ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	query := `
		SELECT 
			id, school_name, school_id, exam_name, exam_question_paper_document_path,
			class, exam_date, number_of_questions, total_mark, questions,
			created_at, created_by, updated_at, updated_by
		FROM academic_assessments 
		WHERE id = $1 AND deleted_at IS NULL;
	`

	var asm dto.AcademicAssessmentDTO
	var questionsRaw string
	var createdAt, updatedAt time.Time

	err := sqlDB.QueryRow(query, assessmentID).Scan(
		&asm.ID, &asm.SchoolName, &asm.SchoolID, &asm.ExamName, &asm.ExamQuestionPaperDocumentPath,
		&asm.Class, &asm.ExamDate, &asm.NumberOfQuestions, &asm.TotalMark, &questionsRaw,
		&createdAt, &asm.CreatedBy, &updatedAt, &asm.UpdatedBy,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Academic assessment not found"})
		return
	}

	_ = json.Unmarshal([]byte(questionsRaw), &asm.Questions)
	asm.CreatedAt = createdAt.Format(time.RFC3339)
	asm.UpdatedAt = updatedAt.Format(time.RFC3339)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"assessment": asm,
	})
}

// GetAssessmentStudents returns all student evaluation records for an assessment, including their answer sheet grid
func (h *AcademicAssessmentHandler) GetAssessmentStudents(c *gin.Context) {
	assessmentID := c.Param("id")
	if assessmentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Assessment ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	statusFilter := c.Query("status")
	query := `
		SELECT 
			id, assessment_id, student_id, student_name, school_id, school_name,
			class, exam_name, exam_date, grid_data, total_marks, obtained_marks,
			percentage, status, debar_reason, evaluator_notes,
			created_at, created_by, updated_at, updated_by
		FROM student_academic_evaluations
		WHERE assessment_id = $1 AND deleted_at IS NULL
	`
	var args []interface{}
	args = append(args, assessmentID)

	if statusFilter != "" && statusFilter != "All" {
		query += " AND status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY student_name ASC;"

	rows, err := sqlDB.Query(query, args...)
	if err != nil {
		log.Printf("Error fetching assessment students: %v", err)
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch student evaluations: " + err.Error()})
		return
	}
	defer rows.Close()

	var students []dto.StudentAcademicEvaluationDTO
	for rows.Next() {
		var s dto.StudentAcademicEvaluationDTO
		var gridRaw string
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&s.ID, &s.AssessmentID, &s.StudentID, &s.StudentName, &s.SchoolID, &s.SchoolName,
			&s.Class, &s.ExamName, &s.ExamDate, &gridRaw, &s.TotalMarks, &s.ObtainedMarks,
			&s.Percentage, &s.Status, &s.DebarReason, &s.EvaluatorNotes,
			&createdAt, &s.CreatedBy, &updatedAt, &s.UpdatedBy,
		)
		if err != nil {
			continue
		}

		_ = json.Unmarshal([]byte(gridRaw), &s.GridData)
		s.CreatedAt = createdAt.Format(time.RFC3339)
		s.UpdatedAt = updatedAt.Format(time.RFC3339)

		students = append(students, s)
	}

	if students == nil {
		students = []dto.StudentAcademicEvaluationDTO{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"students": students,
		"count":    len(students),
	})
}

// EvaluateStudentAssessment saves the evaluated marks for each question in the grid
func (h *AcademicAssessmentHandler) EvaluateStudentAssessment(c *gin.Context) {
	assessmentID := c.Param("id")
	var req dto.EvaluateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid evaluation request body: " + err.Error()})
		return
	}

	if req.StudentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Student ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	// Calculate total obtained marks and total marks from grid data
	obtainedMarks := 0.0
	totalMarks := 0.0
	for _, row := range req.GridData {
		obtainedMarks += row.AnswerMark
		totalMarks += row.QuestionAllocatedMark
	}

	percentage := 0.0
	if totalMarks > 0 {
		percentage = (obtainedMarks / totalMarks) * 100.0
		if percentage > 100.0 {
			percentage = 100.0
		}
	}

	gridJSON, err := json.Marshal(req.GridData)
	if err != nil {
		gridJSON = []byte("[]")
	}

	now := time.Now()
	_, schoolName, userEmail := h.resolveSchoolInfo(c)
	_ = schoolName

	updateQuery := `
		UPDATE student_academic_evaluations
		SET 
			grid_data = $1,
			obtained_marks = $2,
			percentage = $3,
			status = 'Evaluated',
			evaluator_notes = $4,
			updated_at = $5,
			updated_by = $6
		WHERE assessment_id = $7 AND student_id = $8 AND deleted_at IS NULL;
	`

	res, err := sqlDB.Exec(updateQuery, string(gridJSON), obtainedMarks, percentage, req.EvaluatorNotes, now, userEmail, assessmentID, req.StudentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update evaluation: " + err.Error()})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Evaluation record not found for student in this assessment"})
		return
	}

	// Also update StudentDetails table
	_, _ = sqlDB.Exec(
		`UPDATE "StudentDetails" 
		 SET "assessment_grid" = $1, "assessment_status" = 'Evaluated' 
		 WHERE id = $2;`,
		string(gridJSON), req.StudentID,
	)

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        fmt.Sprintf("Evaluation saved: %.1f / %.1f (%.1f%%)", obtainedMarks, totalMarks, percentage),
		"obtained_marks": obtainedMarks,
		"total_marks":    totalMarks,
		"percentage":     percentage,
		"status":         "Evaluated",
	})
}

// PromoteStudentAssessment marks student as Promoted and optionally advances grade
func (h *AcademicAssessmentHandler) PromoteStudentAssessment(c *gin.Context) {
	assessmentID := c.Param("id")
	var req dto.PromoteStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid promote request body: " + err.Error()})
		return
	}

	if req.StudentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Student ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	now := time.Now()
	_, _, userEmail := h.resolveSchoolInfo(c)

	updateQuery := `
		UPDATE student_academic_evaluations
		SET 
			status = 'Promoted',
			evaluator_notes = CASE 
				WHEN $1 <> '' THEN evaluator_notes || ' | Promoted: ' || $1 
				ELSE evaluator_notes 
			END,
			updated_at = $2,
			updated_by = $3
		WHERE assessment_id = $4 AND student_id = $5 AND deleted_at IS NULL;
	`

	res, err := sqlDB.Exec(updateQuery, req.Remarks, now, userEmail, assessmentID, req.StudentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to promote student: " + err.Error()})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student evaluation record not found"})
		return
	}

	// Update StudentDetails table: promote grade if specified
	if req.PromoteNextGrade && req.NextGrade != "" {
		_, _ = sqlDB.Exec(
			`UPDATE "StudentDetails" 
			 SET grade = $1, "academic_status" = 'Promoted', "assessment_status" = 'Promoted' 
			 WHERE id = $2;`,
			req.NextGrade, req.StudentID,
		)
	} else {
		_, _ = sqlDB.Exec(
			`UPDATE "StudentDetails" 
			 SET "academic_status" = 'Promoted', "assessment_status" = 'Promoted' 
			 WHERE id = $1;`,
			req.StudentID,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student promoted successfully",
		"status":  "Promoted",
	})
}

// DebarStudentAssessment marks student as Debarred with a reason
func (h *AcademicAssessmentHandler) DebarStudentAssessment(c *gin.Context) {
	assessmentID := c.Param("id")
	var req dto.DebarStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid debar request body: " + err.Error()})
		return
	}

	if req.StudentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Student ID is required"})
		return
	}

	if strings.TrimSpace(req.DebarReason) == "" {
		req.DebarReason = "Debarred by School Administration for Academic Non-Compliance"
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	now := time.Now()
	_, _, userEmail := h.resolveSchoolInfo(c)

	updateQuery := `
		UPDATE student_academic_evaluations
		SET 
			status = 'Debarred',
			debar_reason = $1,
			updated_at = $2,
			updated_by = $3
		WHERE assessment_id = $4 AND student_id = $5 AND deleted_at IS NULL;
	`

	res, err := sqlDB.Exec(updateQuery, req.DebarReason, now, userEmail, assessmentID, req.StudentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to debar student: " + err.Error()})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Student evaluation record not found"})
		return
	}

	// Update StudentDetails table
	_, _ = sqlDB.Exec(
		`UPDATE "StudentDetails" 
		 SET "assessment_status" = 'Debarred', "academic_status" = 'Debarred' 
		 WHERE id = $1;`,
		req.StudentID,
	)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Student debarred from assessment",
		"status":       "Debarred",
		"debar_reason": req.DebarReason,
	})
}

// BatchAction handles batch promote or debar operations
func (h *AcademicAssessmentHandler) BatchAction(c *gin.Context) {
	assessmentID := c.Param("id")
	var req dto.BatchActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Invalid batch request: " + err.Error()})
		return
	}

	if len(req.StudentIDs) == 0 {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "No student IDs provided"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	now := time.Now()
	_, _, userEmail := h.resolveSchoolInfo(c)

	action := strings.ToLower(req.Action)
	successCount := 0

	for _, sID := range req.StudentIDs {
		if action == "promote" {
			_, err := sqlDB.Exec(
				`UPDATE student_academic_evaluations 
				 SET status = 'Promoted', updated_at = $1, updated_by = $2 
				 WHERE assessment_id = $3 AND student_id = $4 AND deleted_at IS NULL;`,
				now, userEmail, assessmentID, sID,
			)
			if err == nil {
				successCount++
				if req.NextGrade != "" {
					_, _ = sqlDB.Exec(`UPDATE "StudentDetails" SET grade = $1, "academic_status" = 'Promoted' WHERE id = $2;`, req.NextGrade, sID)
				} else {
					_, _ = sqlDB.Exec(`UPDATE "StudentDetails" SET "academic_status" = 'Promoted' WHERE id = $1;`, sID)
				}
			}
		} else if action == "debar" {
			reason := req.DebarReason
			if reason == "" {
				reason = "Debarred via Batch Action by School"
			}
			_, err := sqlDB.Exec(
				`UPDATE student_academic_evaluations 
				 SET status = 'Debarred', debar_reason = $1, updated_at = $2, updated_by = $3 
				 WHERE assessment_id = $4 AND student_id = $5 AND deleted_at IS NULL;`,
				reason, now, userEmail, assessmentID, sID,
			)
			if err == nil {
				successCount++
				_, _ = sqlDB.Exec(`UPDATE "StudentDetails" SET "assessment_status" = 'Debarred' WHERE id = $1;`, sID)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       fmt.Sprintf("Batch %s applied to %d students", action, successCount),
		"updated_count": successCount,
	})
}

// DeleteAcademicAssessment soft deletes an assessment
func (h *AcademicAssessmentHandler) DeleteAcademicAssessment(c *gin.Context) {
	assessmentID := c.Param("id")
	if assessmentID == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Assessment ID is required"})
		return
	}

	sqlDB := database.GetSQLDB()
	if sqlDB == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	now := time.Now()
	_, _, userEmail := h.resolveSchoolInfo(c)

	_, err := sqlDB.Exec(
		`UPDATE academic_assessments 
		 SET deleted_at = $1, deleted_by = $2 
		 WHERE id = $3;`,
		now, userEmail, assessmentID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to delete assessment: " + err.Error()})
		return
	}

	// Soft delete associated student evaluations
	_, _ = sqlDB.Exec(
		`UPDATE student_academic_evaluations 
		 SET deleted_at = $1, deleted_by = $2 
		 WHERE assessment_id = $3;`,
		now, userEmail, assessmentID,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Academic assessment deleted successfully",
	})
}

