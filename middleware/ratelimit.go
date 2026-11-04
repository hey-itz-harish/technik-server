package middleware

import (
	"net/http"
	"sync"
	"time"

	"technik-server/dto"

	"github.com/gin-gonic/gin"
)

type clientRecord struct {
	count     int
	lastReset time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	records map[string]*clientRecord
	limit   int
	window  time.Duration
}

// NewRateLimiter creates an in-memory sliding window rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		records: make(map[string]*clientRecord),
		limit:   limit,
		window:  window,
	}

	// Background cleaner to prevent memory leak from stale IPs
	go func() {
		ticker := time.NewTicker(window * 2)
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for key, rec := range rl.records {
				if now.Sub(rec.lastReset) > window {
					delete(rl.records, key)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

// Middleware creates a Gin middleware for rate limiting by IP and action
func (rl *RateLimiter) Middleware(actionName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := ip + ":" + actionName

		rl.mu.Lock()
		now := time.Now()
		rec, exists := rl.records[key]

		if !exists || now.Sub(rec.lastReset) > rl.window {
			rl.records[key] = &clientRecord{count: 1, lastReset: now}
			rl.mu.Unlock()
			c.Next()
			return
		}

		if rec.count >= rl.limit {
			rl.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, dto.APIError{
				Success: false,
				Error:   "Too many requests. Please wait before trying again.",
			})
			c.Abort()
			return
		}

		rec.count++
		rl.mu.Unlock()
		c.Next()
	}
}
