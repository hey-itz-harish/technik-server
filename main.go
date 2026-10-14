//go:generate go run github.com/steebchen/prisma-client-go generate

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/logger"
	"technik-server/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize structured logger
	logger.InitLogger()

	// 2. Load application environment configuration
	cfg := config.LoadConfig()

	// 3. Set Gin production release mode if running in production
	if os.Getenv("GIN_MODE") == "release" || os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 4. Initialize Prisma Database Client and Raw SQL Pool
	database.InitDB()
	defer database.DisconnectDB()

	// 5. Setup router with CORS, CSRF, Logger, Error Handler, and API routes
	router := routes.SetupRouter(cfg)

	// 6. Configure production HTTP Server with resilient timeouts
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	// 7. Start HTTP Server in background goroutine
	go func() {
		logger.Info("Technik Go Server running on port :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start: %v", err)
			os.Exit(1)
		}
	}()

	// 8. Wait for OS interrupt signal for graceful shutdown (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	logger.Info("Shutdown signal received. Shutting down gracefully...")

	// 9. Shutdown HTTP server with 10s deadline to drain active requests cleanly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown with active connections: %v", err)
	} else {
		logger.Info("Technik Go Server terminated cleanly.")
	}
}

