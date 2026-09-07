package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"technik-server/database"
	"technik-server/dto"
	"technik-server/logger"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
)


type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func sanitizeBody(bodyStr string) string {
	if bodyStr == "" {
		return ""
	}
	// Simple regex or string replacement to mask sensitive parameters like passwords
	re := regexp.MustCompile(`"(password|token|otp)":\s*"[^"]*"`)
	return re.ReplaceAllString(bodyStr, `"$1": "***masked***"`)
}

// RequestLoggerMiddleware logs HTTP requests, latency, request/response payload into AuditLog database
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Capture request body
		var reqBodyStr string
		if c.Request.Body != nil {
			reqBodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
			if len(reqBodyBytes) > 0 {
				reqBodyStr = sanitizeBody(string(reqBodyBytes))
			}
		}

		// Intercept response body
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		if raw != "" {
			path = path + "?" + raw
		}

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errMessage := c.Errors.String()
		userAgent := c.Request.UserAgent()

		userEmailVal, exists := c.Get("userEmail")
		var userEmail string
		if exists {
			userEmail, _ = userEmailVal.(string)
		}

		respBodyStr := sanitizeBody(blw.body.String())

		// Console Structured Logger
		logger.LogRequest(method, path, clientIP, statusCode, latency, errMessage)

		// Async Audit Log Persistence to DB
		go func(m, p, ip, ua, email, reqBody, respBody string, status int, lat time.Duration) {
			if database.Client != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				var optionalParams []db.AuditLogSetParam
				if email != "" {
					optionalParams = append(optionalParams, db.AuditLog.UserEmail.Set(email))
				}
				if ip != "" {
					optionalParams = append(optionalParams, db.AuditLog.IPAddress.Set(ip))
				}
				if ua != "" {
					optionalParams = append(optionalParams, db.AuditLog.UserAgent.Set(ua))
				}
				if reqBody != "" {
					optionalParams = append(optionalParams, db.AuditLog.RequestBody.Set(reqBody))
				}
				if respBody != "" {
					// Truncate response body if overly large for audit storage (max 2000 chars)
					if len(respBody) > 2000 {
						respBody = respBody[:2000] + "...(truncated)"
					}
					optionalParams = append(optionalParams, db.AuditLog.ResponseBody.Set(respBody))
				}

				_, _ = database.Client.AuditLog.CreateOne(
					db.AuditLog.Method.Set(m),
					db.AuditLog.Path.Set(p),
					db.AuditLog.StatusCode.Set(status),
					db.AuditLog.LatencyMs.Set(int(lat.Milliseconds())),
					optionalParams...,
				).Exec(ctx)
			}
		}(method, path, clientIP, userAgent, userEmail, reqBodyStr, respBodyStr, statusCode, latency)
	}
}


// GlobalErrorHandlerMiddleware catches panics and unexpected errors, returning standardized success: false JSON
func GlobalErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				errMsg := fmt.Sprintf("Internal Server Error: %v", err)
				logger.Error("PANIC RECOVERED: %s", errMsg)

				c.JSON(http.StatusInternalServerError, dto.APIError{
					Success: false,
					Error:   errMsg,
				})
				c.Abort()
			}
		}()

		c.Next()

		// If errors exist in Gin context and response hasn't been written
		if len(c.Errors) > 0 && !c.Writer.Written() {
			err := c.Errors.Last()
			logger.Error("Request Error: %v", err.Err)

			c.JSON(http.StatusBadRequest, dto.APIError{
				Success: false,
				Error:   err.Error(),
			})
		}
	}
}
