package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateEntry struct {
	count int
	reset time.Time
}

// RateLimit returns a fixed-window, per-IP limiter. It is process-local, which
// is enough for the public catalog and guest creation limits; a shared store
// can replace it if the API runs multiple instances.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*rateEntry)

	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()

		mu.Lock()
		entry := entries[ip]
		if entry == nil || now.After(entry.reset) {
			entry = &rateEntry{reset: now.Add(window)}
			entries[ip] = entry
		}
		entry.count++
		count := entry.count
		reset := entry.reset
		// Opportunistic cleanup so the map does not grow forever.
		if len(entries) > 10000 {
			for key, value := range entries {
				if now.After(value.reset) {
					delete(entries, key)
				}
			}
		}
		mu.Unlock()

		remaining := limit - count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))

		if count > limit {
			c.Header("Retry-After", strconv.Itoa(int(time.Until(reset).Seconds())+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "RATE_LIMITED"})
			return
		}

		c.Next()
	}
}
