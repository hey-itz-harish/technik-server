package routes

import (
	"net/http"
	"os"
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

	// 3. CORS configuration with dynamic origin matching
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsEnv != "" {
		for _, o := range strings.Split(allowedOriginsEnv, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowedOrigins = append(allowedOrigins, strings.TrimRight(o, "/"))
			}
		}
	}

	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	corsConfig.AllowOriginFunc = func(origin string) bool {
		// Standard production domains
		if origin == "https://www.technikolympiad.com" || origin == "https://technikolympiad.com" {
			return true
		}
		// Custom configured origins from environment
		for _, ao := range allowedOrigins {
			if origin == ao {
				return true
			}
		}
		if cfg.FrontendURL != "" {
			trimmed := strings.TrimRight(cfg.FrontendURL, "/")
			if origin == trimmed || origin == cfg.FrontendURL {
				return true
			}
		}
		// Allow local development and staging preview domains when not in strict production mode
		isDev := os.Getenv("GIN_MODE") != "release" && os.Getenv("APP_ENV") != "production"
		if isDev {
			if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" || origin == "http://localhost:3000" {
				return true
			}
			if strings.HasSuffix(origin, ".vercel.app") || strings.HasSuffix(origin, ".onrender.com") || strings.HasSuffix(origin, ".netlify.app") {
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

	api := r.Group("/api")
	{
		// Admin / Technik Portal routes
		admin := api.Group("/admin")
		{
			// Public admin auth endpoints
			admin.POST("/login", adminHandler.LoginAdmin)
			admin.POST("/verify-otp", adminHandler.VerifyAdminOTP)

			// Admin Authenticated Dashboard Endpoints (Protected by JWT & Role Check)
			adminProtected := admin.Group("")
			adminProtected.Use(middleware.AuthMiddleware(cfg))
			adminProtected.Use(middleware.AdminOnlyMiddleware())
			{
				adminProtected.GET("/stats", adminHandler.GetAdminStats)
				adminProtected.GET("/pride-nominations", adminHandler.GetAdminPrideNominations)
				adminProtected.PATCH("/pride-nominations/:id/status", adminHandler.UpdatePrideNominationStatus)
				adminProtected.POST("/pride-nominations/:id/status", adminHandler.UpdatePrideNominationStatus)
				adminProtected.GET("/olympiad-registrations", adminHandler.GetAdminOlympiadRegistrations)
				adminProtected.GET("/users", adminHandler.GetAdminUsers)
				adminProtected.POST("/users", adminHandler.CreateAdminUser)

				// News & Events Admin Management
				adminProtected.GET("/news-events", adminHandler.GetNewsEvents)
				adminProtected.POST("/news-events", adminHandler.CreateNewsEvent)
				adminProtected.PUT("/news-events/:id", adminHandler.UpdateNewsEvent)
				adminProtected.DELETE("/news-events/:id", adminHandler.DeleteNewsEvent)
			}
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
			auth.POST("/send-otp", authHandler.SendOTP)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
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
			school.POST("/login", authHandler.LoginSchool)
			school.GET("/details", authHandler.GetSchoolDetails)
			school.GET("/profile", authHandler.GetSchoolDetails)
		}

		// Public Contact Us & Inquiries route
		api.POST("/contact", authHandler.SubmitContactEnquiry)

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

