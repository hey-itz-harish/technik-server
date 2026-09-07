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
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 3. CSRF Protection Middleware
	r.Use(middleware.CSRFMiddleware(cfg))

	authHandler := handlers.NewAuthHandler(cfg)

	api := r.Group("/api")
	{
		// Global Auth & OTP routes
		auth := api.Group("/auth")
		{
			auth.GET("/csrf", authHandler.GetCSRFToken)
			auth.GET("/activate", authHandler.ActivateAccount)
			auth.POST("/activate", authHandler.ActivateAccount)
			auth.POST("/send-otp", authHandler.SendOTP)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
			auth.POST("/logout", authHandler.Logout)

			protected := auth.Group("")
			protected.Use(middleware.AuthMiddleware(cfg))
			{
				protected.GET("/me", authHandler.GetMe)
			}
		}

		// School Auth routes
		school := api.Group("/school")
		{
			school.POST("/register", authHandler.RegisterSchool)
			school.POST("/login", authHandler.LoginSchool)
		}

		technikHandler := handlers.NewTechnikHandler(cfg)

		// Technik Pride Award Nomination routes
		pride := api.Group("/technik-pride")
		{
			pride.POST("/nominate", technikHandler.NominateTechnikPride)
			pride.GET("/nominations", technikHandler.GetTechnikPrideNominations)
		}

		// Olympiad Registration routes
		olympiad := api.Group("/olympiad")
		{
			olympiad.POST("/register", technikHandler.RegisterOlympiad)
			olympiad.GET("/registrations", technikHandler.GetOlympiadRegistrations)
		}
	}

	return r
}


