package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	memMu              sync.RWMutex
	memTests           = make(map[string]*dto.OlympiadTestResponse)
	memQuestions       = make(map[string][]dto.OlympiadQuestionDTO)
	memAllocations     = make(map[string]*dto.StudentAllocationResponse) // accessCode -> allocation
	memAllocationsByID = make(map[string]*dto.StudentAllocationResponse) // id -> allocation
	memInitialized     bool
)

func initMemStore() {
	memMu.Lock()
	defer memMu.Unlock()
	if memInitialized {
		return
	}

	testID := "TEST-ROBO-2026"
	now := time.Now()
	startDate := now.Format("2006-01-02")
	endDate := now.Add(14 * 24 * time.Hour).Format("2006-01-02")

	// Sample Questions
	questions := []dto.OlympiadQuestionDTO{
		{
			ID:             "Q-101",
			TestID:         testID,
			QuestionNumber: 1,
			QuestionText:   "Which sensor is primarily used by an autonomous robot to measure distance to an obstacle using sound waves?",
			Options:        []string{"Infrared Sensor", "Ultrasonic Sensor", "Gyroscope Sensor", "Touch Sensor"},
			CorrectOption:  1,
			Marks:          4,
			Explanation:    "Ultrasonic sensors emit high-frequency sound waves and calculate distance based on echo reflection time.",
		},
		{
			ID:             "Q-102",
			TestID:         testID,
			QuestionNumber: 2,
			QuestionText:   "In robotics, what is the primary function of an actuator?",
			Options:        []string{"To process sensor code", "To convert electrical energy into physical mechanical movement", "To supply battery voltage", "To store memory"},
			CorrectOption:  1,
			Marks:          4,
			Explanation:    "Actuators like motors and servos convert electrical signals into motion.",
		},
		{
			ID:             "Q-103",
			TestID:         testID,
			QuestionNumber: 3,
			QuestionText:   "What type of motor allows precise control of angular position without requiring external feedback?",
			Options:        []string{"DC Motor", "Stepper Motor", "Induction Motor", "Brushless DC Motor"},
			CorrectOption:  1,
			Marks:          4,
			Explanation:    "Stepper motors rotate in specific discrete steps, allowing precise open-loop position control.",
		},
		{
			ID:             "Q-104",
			TestID:         testID,
			QuestionNumber: 4,
			QuestionText:   "Which programming concept is used to repeat a robot motion until an ultrasonic sensor detects an obstacle closer than 10 cm?",
			Options:        []string{"If-Else Condition", "While / Until Loop", "Array indexing", "Variable declaration"},
			CorrectOption:  1,
			Marks:          4,
			Explanation:    "A while/until loop continuously executes instructions until the distance condition becomes false.",
		},
		{
			ID:             "Q-105",
			TestID:         testID,
			QuestionNumber: 5,
			QuestionText:   "What is the full form of PWM, commonly used to control robot motor speeds?",
			Options:        []string{"Phase Wave Modulation", "Pulse Width Modulation", "Power Wattage Metric", "Programmable Wire Module"},
			CorrectOption:  1,
			Marks:          4,
			Explanation:    "Pulse Width Modulation (PWM) regulates average power by rapidly switching voltage on and off.",
		},
	}

	test := &dto.OlympiadTestResponse{
		ID:              testID,
		Title:           "National Robotics Olympiad 2026 - Phase 1",
		Track:           "Robotics Olympiad",
		ClassCategory:   "Grade 3 to 5 (Jr Level)",
		StartDate:       startDate,
		EndDate:         endDate,
		StartTime:       "09:00",
		EndTime:         "18:00",
		DurationMinutes: 60,
		TotalMarks:      100,
		PassingMarks:    40,
		Instructions:    "1. Entire screen sharing is mandatory.\n2. Do not switch tabs or exit fullscreen.\n3. Each question carries 4 marks.",
		IsActive:        true,
		TotalQuestions:  len(questions),
		TotalAllocated:  2,
		Questions:       questions,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	memTests[testID] = test
	memQuestions[testID] = questions

	// Sample Candidate 1: Aarav Sharma (Ready to take exam)
	c1 := &dto.StudentAllocationResponse{
		ID:                "ALLOC-101",
		TestID:            testID,
		TestTitle:         test.Title,
		StudentName:       "Aarav Sharma",
		StudentEmail:      "aarav.sharma@dps.edu.in",
		StudentPhone:      "+91 98111 22334",
		SchoolName:        "Delhi Public School, R.K. Puram",
		Grade:             "Grade 5",
		AccessCode:        "TECH-7492",
		ScheduledDate:     startDate,
		AllocatedDuration: 60,
		TimeRemaining:     3600,
		Status:            "NOT_STARTED",
		ResumeAllowed:     false,
		CurrentQuestion:   0,
		Answers:           make(map[string]int),
		ProctorLogs:       []dto.ProctorViolationLog{},
		ScreenShareActive: false,
		TotalQuestions:    len(questions),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	memAllocations["TECH-7492"] = c1
	memAllocationsByID[c1.ID] = c1

	// Sample Candidate 2: Ananya Iyer (Simulates technical error / paused session)
	c2 := &dto.StudentAllocationResponse{
		ID:                "ALLOC-102",
		TestID:            testID,
		TestTitle:         test.Title,
		StudentName:       "Ananya Iyer",
		StudentEmail:      "ananya.iyer@dav.ac.in",
		StudentPhone:      "+91 94444 55667",
		SchoolName:        "DAV Senior Secondary School, Chennai",
		Grade:             "Grade 4",
		AccessCode:        "TECH-3819",
		ScheduledDate:     startDate,
		AllocatedDuration: 60,
		TimeRemaining:     2100,
		Status:            "PAUSED",
		ResumeAllowed:     false,
		CurrentQuestion:   3,
		Answers:           map[string]int{"1": 1, "2": 1},
		ProctorLogs: []dto.ProctorViolationLog{
			{
				Timestamp: now.Add(-15 * time.Minute).Format(time.RFC3339),
				EventType: "SCREEN_SHARE_STOPPED",
				Details:   "Candidate browser crashed / screen sharing disconnected",
			},
		},
		ScreenShareActive: false,
		TotalQuestions:    len(questions),
		AdminNotes:        "Student reported laptop rebooted during test",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	memAllocations["TECH-3819"] = c2
	memAllocationsByID[c2.ID] = c2

	memInitialized = true
}

type OlympiadTestHandler struct {
	cfg *config.Config
}

func NewOlympiadTestHandler(cfg *config.Config) *OlympiadTestHandler {
	initMemStore()
	return &OlympiadTestHandler{cfg: cfg}
}

// Helper to generate a unique candidate access code (e.g. TECH-7384)
func generateAccessCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	return fmt.Sprintf("TECH-%04d", n.Int64()+1000)
}

// -------------------------------------------------------------
// ADMIN ENDPOINTS
// -------------------------------------------------------------

// GetAdminOlympiadTests retrieves all Olympiad tests
func (h *OlympiadTestHandler) GetAdminOlympiadTests(c *gin.Context) {
	initMemStore()

	memMu.RLock()
	var list []dto.OlympiadTestResponse
	for _, t := range memTests {
		list = append(list, *t)
	}
	memMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tests":   list,
	})
}

// CreateOlympiadTest creates a new Olympiad test with questions
func (h *OlympiadTestHandler) CreateOlympiadTest(c *gin.Context) {
	initMemStore()
	var req dto.CreateOlympiadTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	testID := uuid.New().String()
	now := time.Now()

	memMu.Lock()
	testObj := &dto.OlympiadTestResponse{
		ID:              testID,
		Title:           req.Title,
		Track:           req.Track,
		ClassCategory:   req.ClassCategory,
		StartDate:       req.StartDate,
		EndDate:         req.EndDate,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		DurationMinutes: req.DurationMinutes,
		TotalMarks:      req.TotalMarks,
		PassingMarks:    req.PassingMarks,
		Instructions:    req.Instructions,
		IsActive:        req.IsActive,
		TotalQuestions:  len(req.Questions),
		TotalAllocated:  0,
		Questions:       req.Questions,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	memTests[testID] = testObj
	memQuestions[testID] = req.Questions
	memMu.Unlock()

	// Asynchronously save to DB if reachable
	go func() {
		db, err := database.GetRawDB()
		if err == nil && db != nil {
			_, _ = db.Exec(`
				INSERT INTO olympiad_tests (
					id, title, track, class_category, start_date, end_date, start_time, end_time,
					duration_minutes, total_marks, passing_marks, instructions, is_active, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`, testID, req.Title, req.Track, req.ClassCategory, req.StartDate, req.EndDate, req.StartTime, req.EndTime,
				req.DurationMinutes, req.TotalMarks, req.PassingMarks, req.Instructions, req.IsActive, now, now)
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Olympiad Test created successfully",
		"testId":  testID,
	})
}

// GetOlympiadTestByID retrieves test metadata along with full questions list
func (h *OlympiadTestHandler) GetOlympiadTestByID(c *gin.Context) {
	initMemStore()
	testID := c.Param("id")

	memMu.RLock()
	test, exists := memTests[testID]
	questions := memQuestions[testID]
	memMu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Olympiad test not found"})
		return
	}

	resp := *test
	resp.Questions = questions
	resp.TotalQuestions = len(questions)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"test":    resp,
	})
}

// UpdateOlympiadTest updates test details and questions
func (h *OlympiadTestHandler) UpdateOlympiadTest(c *gin.Context) {
	initMemStore()
	testID := c.Param("id")
	var req dto.CreateOlympiadTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	now := time.Now()
	memMu.Lock()
	if t, ok := memTests[testID]; ok {
		t.Title = req.Title
		t.Track = req.Track
		t.ClassCategory = req.ClassCategory
		t.StartDate = req.StartDate
		t.EndDate = req.EndDate
		t.StartTime = req.StartTime
		t.EndTime = req.EndTime
		t.DurationMinutes = req.DurationMinutes
		t.TotalMarks = req.TotalMarks
		t.PassingMarks = req.PassingMarks
		t.Instructions = req.Instructions
		t.IsActive = req.IsActive
		t.UpdatedAt = now
		if len(req.Questions) > 0 {
			t.Questions = req.Questions
			t.TotalQuestions = len(req.Questions)
			memQuestions[testID] = req.Questions
		}
	}
	memMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Olympiad Test updated successfully",
	})
}

// DeleteOlympiadTest deletes an Olympiad test
func (h *OlympiadTestHandler) DeleteOlympiadTest(c *gin.Context) {
	initMemStore()
	testID := c.Param("id")

	memMu.Lock()
	delete(memTests, testID)
	delete(memQuestions, testID)
	memMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Olympiad Test deleted successfully",
	})
}

// UploadOlympiadQuestions adds or replaces questions in bulk
func (h *OlympiadTestHandler) UploadOlympiadQuestions(c *gin.Context) {
	initMemStore()
	testID := c.Param("id")
	var req dto.BulkUploadQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	memMu.Lock()
	memQuestions[testID] = req.Questions
	if t, ok := memTests[testID]; ok {
		t.Questions = req.Questions
		t.TotalQuestions = len(req.Questions)
	}
	memMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        fmt.Sprintf("Uploaded %d questions successfully", len(req.Questions)),
		"totalQuestions": len(req.Questions),
	})
}

// GetOlympiadTestStudents returns all student allocations for a test with proctoring stats
func (h *OlympiadTestHandler) GetOlympiadTestStudents(c *gin.Context) {
	initMemStore()
	testID := c.Param("id")

	memMu.RLock()
	var list []dto.StudentAllocationResponse
	for _, a := range memAllocations {
		if a.TestID == testID || testID == "" {
			list = append(list, *a)
		}
	}
	memMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"students": list,
	})
}

// AllocateStudents assigns one or more students to a test with access code & schedule
func (h *OlympiadTestHandler) AllocateStudents(c *gin.Context) {
	initMemStore()
	testID := c.Param("id")

	var req dto.BulkAllocateStudentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	now := time.Now()
	var allocatedList []dto.StudentAllocationResponse

	memMu.Lock()
	defaultDuration := 60
	totalQ := 5
	if t, ok := memTests[testID]; ok {
		defaultDuration = t.DurationMinutes
		totalQ = t.TotalQuestions
	}

	for _, s := range req.Students {
		allocID := uuid.New().String()
		accessCode := generateAccessCode()

		duration := defaultDuration
		if s.AllocatedDuration > 0 {
			duration = s.AllocatedDuration
		}
		timeRemaining := duration * 60

		item := &dto.StudentAllocationResponse{
			ID:                allocID,
			TestID:            testID,
			StudentName:       s.StudentName,
			StudentEmail:      s.StudentEmail,
			StudentPhone:      s.StudentPhone,
			SchoolName:        s.SchoolName,
			Grade:             s.Grade,
			AccessCode:        accessCode,
			ScheduledDate:     s.ScheduledDate,
			AllocatedDuration: duration,
			TimeRemaining:     timeRemaining,
			Status:            "NOT_STARTED",
			ResumeAllowed:     false,
			Answers:           make(map[string]int),
			TotalQuestions:    totalQ,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		memAllocations[accessCode] = item
		memAllocationsByID[allocID] = item
		allocatedList = append(allocatedList, *item)
	}

	if t, ok := memTests[testID]; ok {
		t.TotalAllocated += len(allocatedList)
	}
	memMu.Unlock()

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("Allocated %d students successfully", len(allocatedList)),
		"allocated": allocatedList,
	})
}

// EditStudentTime allows admin to add minutes or set exact remaining time
func (h *OlympiadTestHandler) EditStudentTime(c *gin.Context) {
	initMemStore()
	allocID := c.Param("allocationId")
	var req dto.EditStudentTimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	memMu.Lock()
	defer memMu.Unlock()

	s, ok := memAllocationsByID[allocID]
	if !ok {
		for _, item := range memAllocations {
			if item.ID == allocID {
				s = item
				ok = true
				break
			}
		}
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Student allocation record not found"})
		return
	}

	var newRemaining int
	if req.ExactRemainingMinutes > 0 {
		newRemaining = req.ExactRemainingMinutes * 60
	} else if req.AdditionalMinutes != 0 {
		newRemaining = s.TimeRemaining + (req.AdditionalMinutes * 60)
		if newRemaining < 0 {
			newRemaining = 0
		}
	} else {
		newRemaining = s.TimeRemaining
	}

	noteAppend := fmt.Sprintf("[%s] Admin adjusted time to %d mins: %s", time.Now().Format("02 Jan 15:04"), newRemaining/60, req.AdminNotes)
	if s.AdminNotes != "" {
		noteAppend = s.AdminNotes + " | " + noteAppend
	}

	s.TimeRemaining = newRemaining
	s.AdminNotes = noteAppend
	s.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          fmt.Sprintf("Time updated successfully to %d minutes", newRemaining/60),
		"newTimeRemaining": newRemaining,
	})
}

// ResumeStudentTest enables resume mode for a student who faced technical issues
func (h *OlympiadTestHandler) ResumeStudentTest(c *gin.Context) {
	initMemStore()
	allocID := c.Param("allocationId")
	var req dto.ResumeStudentTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	memMu.Lock()
	defer memMu.Unlock()

	s, ok := memAllocationsByID[allocID]
	if !ok {
		for _, item := range memAllocations {
			if item.ID == allocID {
				s = item
				ok = true
				break
			}
		}
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Student record not found"})
		return
	}

	newStatus := s.Status
	if req.ResumeAllowed {
		newStatus = "RESUME_ALLOWED"
		s.Blocked = false
		s.BlockedReason = ""
	} else if s.Status == "RESUME_ALLOWED" {
		newStatus = "PAUSED"
	}

	noteMsg := fmt.Sprintf("[%s] Admin toggled resumeAllowed=%v: %s", time.Now().Format("02 Jan 15:04"), req.ResumeAllowed, req.AdminNotes)
	if s.AdminNotes != "" {
		noteMsg = s.AdminNotes + " | " + noteMsg
	}

	s.ResumeAllowed = req.ResumeAllowed
	s.Status = newStatus
	s.AdminNotes = noteMsg
	s.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Student resume permission updated successfully",
		"resumeAllowed": req.ResumeAllowed,
		"status":        newStatus,
	})
}

// ResetStudentTest resets student test to initial state (NOT_STARTED)
func (h *OlympiadTestHandler) ResetStudentTest(c *gin.Context) {
	initMemStore()
	allocID := c.Param("allocationId")
	var req dto.ResetStudentTestRequest
	_ = c.ShouldBindJSON(&req)

	memMu.Lock()
	defer memMu.Unlock()

	s, ok := memAllocationsByID[allocID]
	if !ok {
		for _, item := range memAllocations {
			if item.ID == allocID {
				s = item
				ok = true
				break
			}
		}
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Student record not found"})
		return
	}

	freshRemaining := s.AllocatedDuration * 60
	noteMsg := fmt.Sprintf("[%s] Admin full reset: %s", time.Now().Format("02 Jan 15:04"), req.AdminNotes)
	if s.AdminNotes != "" {
		noteMsg = s.AdminNotes + " | " + noteMsg
	}

	s.Status = "NOT_STARTED"
	s.TimeRemaining = freshRemaining
	s.CurrentQuestion = 0
	s.Answers = make(map[string]int)
	s.ProctorLogs = []dto.ProctorViolationLog{}
	s.ScreenShareActive = false
	s.ResumeAllowed = false
	s.StartedAt = nil
	s.CompletedAt = nil
	s.Score = 0
	s.AdminNotes = noteMsg
	s.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student test session has been completely reset. Candidate can start fresh.",
	})
}

// -------------------------------------------------------------
// STUDENT EXAM PROCTORING ENDPOINTS
// -------------------------------------------------------------

// VerifyExamAccess verifies student's Access Code and returns test rules, state & questions
func (h *OlympiadTestHandler) VerifyExamAccess(c *gin.Context) {
	initMemStore()
	var req dto.StudentExamVerifyRequest
	_ = c.ShouldBindJSON(&req)

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))
	if code == "" {
		code = strings.TrimSpace(strings.ToUpper(c.Query("accessCode")))
	}
	if code == "" {
		code = strings.TrimSpace(strings.ToUpper(c.Query("code")))
	}
	if code == "" {
		code = strings.TrimSpace(strings.ToUpper(c.PostForm("accessCode")))
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Access code is required"})
		return
	}

	memMu.Lock()
	alloc, exists := memAllocations[code]
	var test *dto.OlympiadTestResponse
	var questions []dto.OlympiadQuestionDTO
	if exists {
		test = memTests[alloc.TestID]
		questions = memQuestions[alloc.TestID]

		// Auto-resume logic: if candidate had an active session (IN_PROGRESS, PAUSED, or AutoResume)
		// but was NOT blocked for tab-switching, and has time remaining:
		// auto-enable resume so they can seamlessly continue after window closed / power off / network loss!
		if !alloc.Blocked && alloc.Status != "SUBMITTED" && alloc.Status != "BLOCKED" {
			if alloc.Status == "IN_PROGRESS" || alloc.Status == "PAUSED" || alloc.AutoResume {
				if alloc.TimeRemaining > 0 {
					alloc.ResumeAllowed = true
					alloc.Status = "RESUME_ALLOWED"
					alloc.AutoResume = true
				}
			}
		}
	}
	memMu.Unlock()

	if !exists || test == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Invalid Access Code. Please verify your Olympiad test code.",
		})
		return
	}

	savedAnswers := alloc.Answers
	if savedAnswers == nil {
		savedAnswers = make(map[string]int)
	}

	resp := dto.StudentExamVerifyResponse{
		Success:           true,
		Message:           "Candidate verified successfully",
		AllocationID:      alloc.ID,
		TestID:            test.ID,
		TestTitle:         test.Title,
		Track:             test.Track,
		ClassCategory:     test.ClassCategory,
		Instructions:      test.Instructions,
		StartDate:         test.StartDate,
		EndDate:           test.EndDate,
		StartTime:         test.StartTime,
		EndTime:           test.EndTime,
		StudentName:       alloc.StudentName,
		StudentEmail:      alloc.StudentEmail,
		SchoolName:        alloc.SchoolName,
		Grade:             alloc.Grade,
		AccessCode:        alloc.AccessCode,
		Status:            alloc.Status,
		ResumeAllowed:     alloc.ResumeAllowed,
		Blocked:           alloc.Blocked,
		BlockedReason:     alloc.BlockedReason,
		AutoResume:        alloc.AutoResume,
		AllocatedDuration: alloc.AllocatedDuration,
		TimeRemaining:     alloc.TimeRemaining,
		TotalQuestions:    len(questions),
		CurrentQuestion:   alloc.CurrentQuestion,
		SavedAnswers:      savedAnswers,
		Questions:         questions,
	}

	c.JSON(http.StatusOK, resp)
}

// StartExamSession marks exam in progress and validates screen share
func (h *OlympiadTestHandler) StartExamSession(c *gin.Context) {
	initMemStore()
	var req dto.StudentExamStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))
	now := time.Now()

	memMu.Lock()
	if a, ok := memAllocations[code]; ok && a.Status != "SUBMITTED" {
		a.Status = "IN_PROGRESS"
		a.ScreenShareActive = req.ScreenShareActive
		if a.StartedAt == nil {
			a.StartedAt = &now
		}
		a.LastHeartbeatAt = &now
		a.UpdatedAt = now
	}
	memMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Proctored exam session initiated",
	})
}

// ExamHeartbeat syncs remaining seconds, answers, screen share status and violations
func (h *OlympiadTestHandler) ExamHeartbeat(c *gin.Context) {
	initMemStore()
	var req dto.StudentExamHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))

	memMu.Lock()
	defer memMu.Unlock()

	alloc, exists := memAllocations[code]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Session not found"})
		return
	}

	if alloc.Status == "SUBMITTED" {
		c.JSON(http.StatusOK, dto.StudentExamHeartbeatResponse{
			Success: true,
			Status:  "SUBMITTED",
			Message: "Test already submitted",
		})
		return
	}

	if len(req.Violations) > 0 {
		alloc.ProctorLogs = append(alloc.ProctorLogs, req.Violations...)
	}
	alloc.Answers = req.Answers

	savedRemaining := req.TimeRemaining
	if alloc.TimeRemaining > req.TimeRemaining+60 {
		// Admin extended time!
		savedRemaining = alloc.TimeRemaining
	} else {
		alloc.TimeRemaining = savedRemaining
	}

	alloc.CurrentQuestion = req.CurrentQuestion
	alloc.ScreenShareActive = req.ScreenShareActive

	// Update live screen snapshot for admin proctoring monitor
	if req.ScreenSnapshot != "" {
		alloc.ScreenSnapshot = req.ScreenSnapshot
	}

	// If candidate became blocked during session
	if req.Blocked {
		alloc.Blocked = true
		alloc.Status = "BLOCKED"
		alloc.BlockedReason = req.BlockedReason
		alloc.ResumeAllowed = false
		alloc.ScreenShareActive = false
	}

	now := time.Now()
	alloc.LastHeartbeatAt = &now
	alloc.UpdatedAt = now

	c.JSON(http.StatusOK, dto.StudentExamHeartbeatResponse{
		Success:       true,
		Status:        alloc.Status,
		TimeRemaining: savedRemaining,
		ResumeAllowed: alloc.ResumeAllowed,
		Blocked:       alloc.Blocked,
	})
}

// InterruptExamSession handles accidental browser close, power drop, or network disconnect
func (h *OlympiadTestHandler) InterruptExamSession(c *gin.Context) {
	initMemStore()
	var req dto.StudentExamInterruptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))
	memMu.Lock()
	defer memMu.Unlock()

	alloc, exists := memAllocations[code]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Candidate session not found"})
		return
	}

	if alloc.Status == "SUBMITTED" || alloc.Blocked {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"status":  alloc.Status,
			"message": "Status preserved",
		})
		return
	}

	now := time.Now()
	alloc.Status = "RESUME_ALLOWED"
	alloc.ResumeAllowed = true
	alloc.AutoResume = true
	alloc.ScreenShareActive = false
	if req.TimeRemaining > 0 {
		alloc.TimeRemaining = req.TimeRemaining
	}
	if len(req.Answers) > 0 {
		alloc.Answers = req.Answers
	}
	alloc.CurrentQuestion = req.CurrentQuestion
	alloc.UpdatedAt = now

	reason := req.Reason
	if reason == "" {
		reason = "CHROME_WINDOW_CLOSED"
	}
	alloc.AdminNotes = fmt.Sprintf("Session interrupted (%s) at %s. Auto-resume flag activated.", reason, now.Format("15:04:05"))

	alloc.ProctorLogs = append(alloc.ProctorLogs, dto.ProctorViolationLog{
		Timestamp: now.Format(time.RFC3339),
		EventType: "SESSION_INTERRUPT",
		Details:   fmt.Sprintf("Browser window closed or connection lost (%s). Auto-resume permitted.", reason),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Session interruption marked. Auto-resume enabled.",
		"resumeAllowed": true,
		"status":        alloc.Status,
	})
}

// BlockExamSession locks candidate when tab switch or cheat event occurs
func (h *OlympiadTestHandler) BlockExamSession(c *gin.Context) {
	initMemStore()
	var req dto.StudentExamBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))
	memMu.Lock()
	defer memMu.Unlock()

	alloc, exists := memAllocations[code]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Candidate session not found"})
		return
	}

	now := time.Now()
	alloc.Blocked = true
	alloc.Status = "BLOCKED"
	alloc.BlockedReason = req.Reason
	alloc.ResumeAllowed = false
	alloc.AutoResume = false
	alloc.ScreenShareActive = false
	alloc.UpdatedAt = now
	alloc.AdminNotes = fmt.Sprintf("BLOCKED: %s at %s. Candidate navigated away or opened new tab.", req.Reason, now.Format("15:04:05"))

	alloc.ProctorLogs = append(alloc.ProctorLogs, dto.ProctorViolationLog{
		Timestamp: now.Format(time.RFC3339),
		EventType: "TAB_SWITCH_BLOCKED",
		Details:   fmt.Sprintf("Candidate opened or moved to another tab (%s). Exam permanently blocked until timer expires or admin unlocks.", req.Details),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"blocked": true,
		"status":  "BLOCKED",
		"message": "Candidate blocked due to security violation",
	})
}


// SaveExamAnswer saves candidate's answer for an individual question immediately
func (h *OlympiadTestHandler) SaveExamAnswer(c *gin.Context) {
	initMemStore()
	var req dto.StudentSaveAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))

	memMu.Lock()
	defer memMu.Unlock()

	alloc, exists := memAllocations[code]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Allocation not found"})
		return
	}

	if alloc.Answers == nil {
		alloc.Answers = make(map[string]int)
	}

	qKey := fmt.Sprintf("%d", req.QuestionNumber)
	if req.SelectedOption >= 0 {
		alloc.Answers[qKey] = req.SelectedOption
	} else {
		delete(alloc.Answers, qKey)
	}
	alloc.TimeRemaining = req.TimeRemaining
	now := time.Now()
	alloc.LastHeartbeatAt = &now
	alloc.UpdatedAt = now

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Answer saved",
	})
}

// LogProctorEvent logs security events like tab switches, screenshare stopped, full-screen exit
func (h *OlympiadTestHandler) LogProctorEvent(c *gin.Context) {
	initMemStore()
	var req dto.StudentLogEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))

	memMu.Lock()
	defer memMu.Unlock()

	alloc, exists := memAllocations[code]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Allocation not found"})
		return
	}

	if req.Event.Timestamp == "" {
		req.Event.Timestamp = time.Now().Format(time.RFC3339)
	}
	alloc.ProctorLogs = append(alloc.ProctorLogs, req.Event)
	alloc.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Proctoring event recorded",
	})
}

// SubmitExam finalizes test submission, calculates score, and saves timestamp
func (h *OlympiadTestHandler) SubmitExam(c *gin.Context) {
	initMemStore()
	var req dto.StudentSubmitExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))

	memMu.Lock()
	defer memMu.Unlock()

	alloc, exists := memAllocations[code]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Invalid access code"})
		return
	}

	questions := memQuestions[alloc.TestID]
	test := memTests[alloc.TestID]
	score := 0
	totalMarks := 0
	passingMarks := 0
	totalQuestions := len(questions)

	if test != nil {
		passingMarks = test.PassingMarks
	}
	for _, q := range questions {
		totalMarks += q.Marks
		qKey := fmt.Sprintf("%d", q.QuestionNumber)
		if selected, found := req.Answers[qKey]; found && selected == q.CorrectOption {
			score += q.Marks
		}
	}

	passed := score >= passingMarks
	now := time.Now()
	alloc.Status = "SUBMITTED"
	alloc.Answers = req.Answers
	alloc.Score = score
	alloc.TotalMarks = totalMarks
	alloc.TotalQuestions = totalQuestions
	alloc.CompletedAt = &now
	alloc.ScreenShareActive = false
	alloc.UpdatedAt = now

	// Auto-issue certificate if candidate passed
	certTemplate := ""
	if test != nil {
		certTemplate = test.CertificateTemplate
		if passed && certTemplate != "" {
			alloc.CertificateIssued = true
		}
	}

	submissionID := fmt.Sprintf("SUB-%s-%d", strings.ReplaceAll(code, "-", ""), now.Unix()%100000)

	c.JSON(http.StatusOK, dto.StudentSubmitExamResponse{
		Success:             true,
		Message:             "Olympiad test submitted successfully!",
		SubmissionID:        submissionID,
		StudentName:         alloc.StudentName,
		SchoolName:          alloc.SchoolName,
		Grade:               alloc.Grade,
		AccessCode:          code,
		TotalQuestions:      totalQuestions,
		AttemptedCount:      len(req.Answers),
		Score:               score,
		TotalMarks:          totalMarks,
		PassingMarks:        passingMarks,
		Passed:              passed,
		CertificateTemplate: certTemplate,
		CompletedAt:         now,
	})
}

// GetExamResult returns the result + certificate for a submitted exam
func (h *OlympiadTestHandler) GetExamResult(c *gin.Context) {
	initMemStore()
	var req dto.ExamResultRequest
	_ = c.ShouldBindJSON(&req)
	code := strings.TrimSpace(strings.ToUpper(req.AccessCode))
	if code == "" {
		code = strings.TrimSpace(strings.ToUpper(c.Query("accessCode")))
	}
	if code == "" {
		code = strings.TrimSpace(strings.ToUpper(c.Param("code")))
	}
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Access code is required"})
		return
	}

	memMu.RLock()
	alloc, exists := memAllocations[code]
	var test *dto.OlympiadTestResponse
	if exists {
		test = memTests[alloc.TestID]
	}
	memMu.RUnlock()

	if !exists || alloc.Status != "SUBMITTED" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Result not found or exam not yet submitted"})
		return
	}

	testTitle := ""
	track := ""
	totalMarks := 0
	passingMarks := 0
	certTemplate := ""
	if test != nil {
		testTitle = test.Title
		track = test.Track
		totalMarks = test.TotalMarks
		passingMarks = test.PassingMarks
		certTemplate = test.CertificateTemplate
	}
	if totalMarks == 0 {
		totalMarks = alloc.TotalMarks
	}

	attempted := len(alloc.Answers)
	passed := alloc.Score >= passingMarks
	subID := fmt.Sprintf("SUB-%s", strings.ReplaceAll(code, "-", ""))
	completedAt := time.Now()
	if alloc.CompletedAt != nil {
		completedAt = *alloc.CompletedAt
	}

	c.JSON(http.StatusOK, dto.ExamResultResponse{
		Success:             true,
		StudentName:         alloc.StudentName,
		StudentEmail:        alloc.StudentEmail,
		SchoolName:          alloc.SchoolName,
		Grade:               alloc.Grade,
		AccessCode:          code,
		TestTitle:           testTitle,
		Track:               track,
		Score:               alloc.Score,
		TotalMarks:          totalMarks,
		PassingMarks:        passingMarks,
		AttemptedCount:      attempted,
		TotalQuestions:      alloc.TotalQuestions,
		Passed:              passed,
		SubmissionID:        subID,
		CertificateTemplate: certTemplate,
		CertificateIssued:   alloc.CertificateIssued,
		CompletedAt:         completedAt,
	})
}

// UploadCertTemplate allows admin to upload certificate background image for a test
func (h *OlympiadTestHandler) UploadCertTemplate(c *gin.Context) {
	initMemStore()
	var req dto.UploadCertificateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	memMu.Lock()
	if t, ok := memTests[req.TestID]; ok {
		t.CertificateTemplate = req.CertificateTemplate
		t.UpdatedAt = time.Now()
	}
	memMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Certificate template uploaded successfully",
		"testId":  req.TestID,
	})
}

// IssueCertificate allows admin to manually issue or revoke a student certificate
func (h *OlympiadTestHandler) IssueCertificate(c *gin.Context) {
	initMemStore()
	var req dto.IssueCertificateRequest
	_ = c.ShouldBindJSON(&req)
	if req.AllocationID == "" {
		req.AllocationID = c.Param("allocationId")
	}
	if req.AllocationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Allocation ID is required"})
		return
	}

	memMu.Lock()
	defer memMu.Unlock()

	s, ok := memAllocationsByID[req.AllocationID]
	if !ok {
		for _, item := range memAllocations {
			if item.ID == req.AllocationID {
				s = item
				ok = true
				break
			}
		}
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Student record not found"})
		return
	}

	s.CertificateIssued = req.CertificateIssued
	s.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "Certificate status updated",
		"certificateIssued": req.CertificateIssued,
	})
}
