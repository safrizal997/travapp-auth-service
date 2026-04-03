package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORS_WildcardOrigin(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"*"}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Errorf("expected origin http://example.com, got %s", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected credentials true, got %s", got)
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"http://allowed.com"}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	// Allowed origin
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://allowed.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://allowed.com" {
		t.Errorf("expected origin http://allowed.com, got %s", got)
	}

	// Disallowed origin
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Origin", "http://notallowed.com")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin for disallowed origin, got %s", got)
	}
}

func TestCORS_OptionsPreflight(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"*"}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("expected max-age 86400, got %s", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Allow-Methods header")
	}
}
