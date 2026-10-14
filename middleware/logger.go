package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
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
	re := regexp.MustCompile(`"(password|token|otp)":\s*"[^"]*"`)
	return re.ReplaceAllString(bodyStr, `"$1": "***masked***"`)
}

type auditLogEntry struct {
	method       string
	path         string
	ip           string
	userAgent    string
	userEmail    string
	requestBody  string
	responseBody string
	statusCode   int
	latency      time.Duration
}

// Bounded queue buffer to prevent database thread starvation under peak loads
var auditLogChan = make(chan auditLogEntry, 1000)

func init() {
	go auditLogWorker()
}

func auditLogWorker() {
	for entry := range auditLogChan {
		if database.Client == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var optionalParams []db.AuditLogSetParam
		if entry.userEmail != "" {
			optionalParams = append(optionalParams, db.AuditLog.UserEmail.Set(entry.userEmail))
		}
		if entry.ip != "" {
			optionalParams = append(optionalParams, db.AuditLog.IPAddress.Set(entry.ip))
		}
		if entry.userAgent != "" {
			optionalParams = append(optionalParams, db.AuditLog.UserAgent.Set(entry.userAgent))
		}
		if entry.requestBody != "" {
			optionalParams = append(optionalParams, db.AuditLog.RequestBody.Set(entry.requestBody))
		}
		if entry.responseBody != "" {
			resp := entry.responseBody
			if len(resp) > 2000 {
				resp = resp[:2000] + "...(truncated)"
			}
			optionalParams = append(optionalParams, db.AuditLog.ResponseBody.Set(resp))
		}

		_, _ = database.Client.AuditLog.CreateOne(
			db.AuditLog.Method.Set(entry.method),
			db.AuditLog.Path.Set(entry.path),
			db.AuditLog.StatusCode.Set(entry.statusCode),
			db.AuditLog.LatencyMs.Set(int(entry.latency.Milliseconds())),
			optionalParams...,
		).Exec(ctx)
		cancel()
	}
}

// RequestLoggerMiddleware logs HTTP requests, latency, request/response payload into AuditLog database
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Capture request body safely without memory leaks on large multipart file uploads
		var reqBodyStr string
		contentType := c.ContentType()
		isMultipart := strings.HasPrefix(contentType, "multipart/form-data")
		isLarge := c.Request.ContentLength > 64*1024 // 64 KB threshold

		if c.Request.Body != nil && !isMultipart && !isLarge {
			reqBodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
			if len(reqBodyBytes) > 0 {
				reqBodyStr = sanitizeBody(string(reqBodyBytes))
			}
		} else if isMultipart {
			reqBodyStr = "[multipart/form-data payload omitted from audit log]"
		} else if isLarge {
			reqBodyStr = "[large payload >64KB omitted from audit log]"
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

		// Non-blocking enqueue to background audit worker
		select {
		case auditLogChan <- auditLogEntry{
			method:       method,
			path:         path,
			ip:           clientIP,
			userAgent:    userAgent,
			userEmail:    userEmail,
			requestBody:  reqBodyStr,
			responseBody: respBodyStr,
			statusCode:   statusCode,
			latency:      latency,
		}:
		default:
			// Buffer full: drop audit log gracefully without impacting live HTTP response latency
		}
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
