package routes

import (
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

	// 2. CORS configuration
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"https://www.technikolympiad.com",
			"https://technikolympiad.com",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:3000",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 3. CSRF Protection Middleware
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
			admin.POST("/login", adminHandler.LoginAdmin)
			admin.POST("/verify-otp", adminHandler.VerifyAdminOTP)

			// Admin Authenticated Dashboard Endpoints
			admin.GET("/stats", adminHandler.GetAdminStats)
			admin.GET("/pride-nominations", adminHandler.GetAdminPrideNominations)
			admin.PATCH("/pride-nominations/:id/status", adminHandler.UpdatePrideNominationStatus)
			admin.POST("/pride-nominations/:id/status", adminHandler.UpdatePrideNominationStatus)
			admin.GET("/olympiad-registrations", adminHandler.GetAdminOlympiadRegistrations)
			admin.GET("/users", adminHandler.GetAdminUsers)
			admin.POST("/users", adminHandler.CreateAdminUser)
		}

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

		// School Auth routes (public)
		school := api.Group("/school")
		{
			school.POST("/register", authHandler.RegisterSchool)
			school.POST("/login", authHandler.LoginSchool)
		}

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
	}

	return r
}
