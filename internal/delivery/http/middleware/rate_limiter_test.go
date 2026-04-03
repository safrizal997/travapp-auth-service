package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter_Passthrough(t *testing.T) {
	r := gin.New()
	r.Use(RateLimiter())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAbortWithTooManyRequests(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		AbortWithTooManyRequests(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 429 {
		t.Errorf("expected 429, got %d", w.Code)
	}
}
