package routes

import (
	"net/http"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/handlers"
	"technik-server/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config) *gin.Engine {
	r := gin.New()

	// Set trusted proxies to local loopback interface to eliminate warning
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// 1. Structured Logging & Global Recovery / Error Handler Middleware
	r.Use(middleware.RequestLoggerMiddleware())
	r.Use(middleware.GlobalErrorHandlerMiddleware())

	// 2. Health check endpoints for Docker / Kubernetes / Cloud Load Balancers
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "technik-server",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
	r.GET("/health", healthHandler)
	r.GET("/api/health", healthHandler)

	// 3. CORS configuration: Dynamically allow any localhost / 127.0.0.1 port and production domains
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	corsConfig.AllowOriginFunc = func(origin string) bool {
		if strings.HasPrefix(origin, "http://localhost:") ||
			strings.HasPrefix(origin, "http://127.0.0.1:") ||
			origin == "http://localhost" ||
			origin == "http://127.0.0.1" ||
			origin == "https://www.technikolympiad.com" ||
			origin == "https://technikolympiad.com" ||
			strings.HasSuffix(origin, ".vercel.app") ||
			strings.HasSuffix(origin, ".onrender.com") ||
			strings.HasSuffix(origin, ".netlify.app") {
			return true
		}
		if cfg.FrontendURL != "" {
			trimmed := strings.TrimRight(cfg.FrontendURL, "/")
			if origin == trimmed || origin == cfg.FrontendURL {
				return true
			}
		}
		return false
	}
	r.Use(cors.New(corsConfig))

	// 4. CSRF Protection Middleware
	r.Use(middleware.CSRFMiddleware(cfg))

	// Static file serving for uploaded certificates and documents
	r.Static("/uploads", "./uploads")

	authHandler := handlers.NewAuthHandler(cfg)
	technikHandler := handlers.NewTechnikHandler(cfg)
	adminHandler := handlers.NewAdminHandler(cfg)
	olympiadTestHandler := handlers.NewOlympiadTestHandler(cfg)
	assessmentHandler := handlers.NewAcademicAssessmentHandler(cfg)

	// Rate limiters for brute-force and abuse protection
	authLimiter := middleware.NewRateLimiter(5, 1*time.Minute)
	otpSendLimiter := middleware.NewRateLimiter(3, 1*time.Minute)
	contactLimiter := middleware.NewRateLimiter(3, 1*time.Minute)

	api := r.Group("/api")
	{
		// Admin / Technik Portal routes
		admin := api.Group("/admin")
		{
			admin.POST("/login", authLimiter.Middleware("admin-login"), adminHandler.LoginAdmin)
			admin.POST("/verify-otp", authLimiter.Middleware("admin-verify-otp"), adminHandler.VerifyAdminOTP)

			// Admin Authenticated Dashboard Endpoints (Protected by JWT & Admin Role)
			adminProtected := admin.Group("")
			adminProtected.Use(middleware.AuthMiddleware(cfg), middleware.RequireAdminRole())
			{
				adminProtected.GET("/stats", adminHandler.GetAdminStats)
				adminProtected.GET("/pride-nominations", adminHandler.GetAdminPrideNominations)
				adminProtected.PATCH("/pride-nominations/:id/status", adminHandler.UpdatePrideNominationStatus)
				adminProtected.POST("/pride-nominations/:id/status", adminHandler.UpdatePrideNominationStatus)
				adminProtected.GET("/olympiad-registrations", adminHandler.GetAdminOlympiadRegistrations)
				adminProtected.GET("/users", adminHandler.GetAdminUsers)
				adminProtected.POST("/users", adminHandler.CreateAdminUser)

				// Olympiad Test Creation, Question Upload & Proctoring Management
				adminProtected.GET("/olympiad-tests", olympiadTestHandler.GetAdminOlympiadTests)
				adminProtected.POST("/olympiad-tests", olympiadTestHandler.CreateOlympiadTest)
				adminProtected.GET("/olympiad-tests/:id", olympiadTestHandler.GetOlympiadTestByID)
				adminProtected.PUT("/olympiad-tests/:id", olympiadTestHandler.UpdateOlympiadTest)
				adminProtected.DELETE("/olympiad-tests/:id", olympiadTestHandler.DeleteOlympiadTest)
				adminProtected.POST("/olympiad-tests/:id/questions", olympiadTestHandler.UploadOlympiadQuestions)
				adminProtected.GET("/olympiad-tests/:id/students", olympiadTestHandler.GetOlympiadTestStudents)
				adminProtected.POST("/olympiad-tests/:id/allocate", olympiadTestHandler.AllocateStudents)
				adminProtected.PATCH("/olympiad-tests/student/:allocationId/time", olympiadTestHandler.EditStudentTime)
				adminProtected.PATCH("/olympiad-tests/student/:allocationId/resume", olympiadTestHandler.ResumeStudentTest)
				adminProtected.POST("/olympiad-tests/student/:allocationId/reset", olympiadTestHandler.ResetStudentTest)
				adminProtected.POST("/olympiad-tests/cert-template", olympiadTestHandler.UploadCertTemplate)
				adminProtected.PATCH("/olympiad-tests/student/:allocationId/certificate", olympiadTestHandler.IssueCertificate)

				// News & Events Admin Management
				adminProtected.GET("/news-events", adminHandler.GetNewsEvents)
				adminProtected.POST("/news-events", adminHandler.CreateNewsEvent)
				adminProtected.PUT("/news-events/:id", adminHandler.UpdateNewsEvent)
				adminProtected.DELETE("/news-events/:id", adminHandler.DeleteNewsEvent)
			}
		}

		// Public Student Proctored Exam Portal Routes
		exam := api.Group("/olympiad-exam")
		{
			exam.POST("/verify", olympiadTestHandler.VerifyExamAccess)
			exam.GET("/result", olympiadTestHandler.GetExamResult)
			exam.POST("/result", olympiadTestHandler.GetExamResult)
			exam.POST("/start", olympiadTestHandler.StartExamSession)
			exam.POST("/heartbeat", olympiadTestHandler.ExamHeartbeat)
			exam.POST("/save-answer", olympiadTestHandler.SaveExamAnswer)
			exam.POST("/log-event", olympiadTestHandler.LogProctorEvent)
			exam.POST("/interrupt", olympiadTestHandler.InterruptExamSession)
			exam.POST("/block", olympiadTestHandler.BlockExamSession)
			exam.POST("/submit", olympiadTestHandler.SubmitExam)
		}

		// Public News & Events endpoint for frontend / cards
		api.GET("/news-events", adminHandler.GetNewsEvents)

		// Global Auth & OTP routes
		auth := api.Group("/auth")
		{
			auth.GET("/csrf", authHandler.GetCSRFToken)
			auth.GET("/activate", authHandler.ActivateAccount)
			auth.POST("/activate", authHandler.ActivateAccount)
			auth.GET("/activation-status", authHandler.GetActivationStatus)
			auth.GET("/resend-activation", authHandler.ResendActivationByToken)
			auth.POST("/resend-activation", authHandler.ResendActivationByEmail)
			auth.POST("/send-otp", otpSendLimiter.Middleware("send-otp"), authHandler.SendOTP)
			auth.POST("/verify-otp", authLimiter.Middleware("verify-otp"), authHandler.VerifyOTP)
			auth.POST("/logout", authHandler.Logout)

			// Microsoft Authenticator (TOTP) MFA setup & status routes
			auth.GET("/mfa/setup", authHandler.GetMfaSetupQR)
			auth.POST("/mfa/verify-setup", authHandler.VerifyMfaSetup)
			auth.GET("/mfa-status", authHandler.GetMfaStatus)

			protected := auth.Group("")
			protected.Use(middleware.AuthMiddleware(cfg))
			{
				protected.GET("/me", authHandler.GetMe)
			}
		}

		// School Auth & Profile routes
		school := api.Group("/school")
		{
			school.POST("/register", authHandler.RegisterSchool)
			school.POST("/login", authLimiter.Middleware("school-login"), authHandler.LoginSchool)
			school.GET("/details", authHandler.GetSchoolDetails)
			school.GET("/profile", authHandler.GetSchoolDetails)
		}

		// Public Contact Us & Inquiries route
		api.POST("/contact", contactLimiter.Middleware("contact"), authHandler.SubmitContactEnquiry)

		// Authenticated School Portal & Technik Competition routes
		portal := api.Group("")
		portal.Use(middleware.AuthMiddleware(cfg))
		{
			portal.GET("/school/students", technikHandler.GetSchoolStudents)
			portal.POST("/school/upload-document", technikHandler.UploadDocument)

			portal.POST("/technik-pride/nominate", technikHandler.NominateTechnikPride)
			portal.GET("/technik-pride/nominations", technikHandler.GetTechnikPrideNominations)
			portal.POST("/technik-pride/upload", technikHandler.UploadDocument)

			portal.POST("/olympiad/register", technikHandler.RegisterOlympiad)
			portal.GET("/olympiad/registrations", technikHandler.GetOlympiadRegistrations)

			// Academic Assessment Setup & Student Evaluation Grid routes
			portal.POST("/school/academic-assessment/create", assessmentHandler.CreateAcademicAssessment)
			portal.GET("/school/academic-assessment", assessmentHandler.GetAcademicAssessments)
			portal.GET("/school/academic-assessment/:id", assessmentHandler.GetAcademicAssessmentByID)
			portal.GET("/school/academic-assessment/:id/students", assessmentHandler.GetAssessmentStudents)
			portal.POST("/school/academic-assessment/:id/evaluate", assessmentHandler.EvaluateStudentAssessment)
			portal.POST("/school/academic-assessment/:id/promote", assessmentHandler.PromoteStudentAssessment)
			portal.POST("/school/academic-assessment/:id/debar", assessmentHandler.DebarStudentAssessment)
			portal.POST("/school/academic-assessment/:id/batch", assessmentHandler.BatchAction)
			portal.DELETE("/school/academic-assessment/:id", assessmentHandler.DeleteAcademicAssessment)
		}

		// Student Portfolio & Skill Compass Additional Data routes
		api.GET("/student/:id/portfolio", technikHandler.GetStudentPortfolio)
		api.POST("/student/:id/portfolio", middleware.AuthMiddleware(cfg), technikHandler.SaveStudentPortfolio)

		// Razorpay Payment Gateway routes
		api.POST("/payment/razorpay/create-order", technikHandler.CreateRazorpayOrder)
		api.POST("/payment/razorpay/verify", technikHandler.VerifyRazorpayPayment)
	}

	return r
}

