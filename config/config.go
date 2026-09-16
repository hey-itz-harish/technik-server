package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	CSRFSecret   string
	CookieDomain string
	CookieSecure bool
	FrontendURL  string
	ZohoHost     string
	ZohoPort     string
	ZohoUser     string
	ZohoPass     string
	ZohoFrom     string
	ResendApiKey string
	ResendFrom   string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Note: .env file not found, reading environment variables from system")
	}

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/technik_db?schema=public")
	jwtSecret := getEnv("JWT_SECRET", "default_jwt_secret_change_me_2026")
	csrfSecret := getEnv("CSRF_SECRET", "default_csrf_secret_change_me_2026")
	cookieDomain := getEnv("COOKIE_DOMAIN", "localhost")
	cookieSecure := getEnv("COOKIE_SECURE", "false") == "true"
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:5173/#")

	zohoHost := getEnv("ZOHO_SMTP_HOST", "smtp.zoho.in")
	zohoPort := getEnv("ZOHO_SMTP_PORT", "587")
	zohoUser := getEnv("ZOHO_SMTP_USER", "")
	zohoPass := getEnv("ZOHO_SMTP_PASS", "")
	zohoFrom := getEnv("ZOHO_FROM_EMAIL", zohoUser)

	resendApiKey := getEnv("RESEND_API_KEY", "")
	resendFrom := getEnv("RESEND_FROM_EMAIL", "Technik Olympiad <onboarding@resend.dev>")

	return &Config{
		Port:         port,
		DatabaseURL:  dbURL,
		JWTSecret:    jwtSecret,
		CSRFSecret:   csrfSecret,
		CookieDomain: cookieDomain,
		CookieSecure: cookieSecure,
		FrontendURL:  frontendURL,
		ZohoHost:     zohoHost,
		ZohoPort:     zohoPort,
		ZohoUser:     zohoUser,
		ZohoPass:     zohoPass,
		ZohoFrom:     zohoFrom,
		ResendApiKey: resendApiKey,
		ResendFrom:   resendFrom,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
