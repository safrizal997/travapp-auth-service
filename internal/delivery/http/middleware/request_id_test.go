package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestID_GenerateNew(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())

	var gotRequestID string
	r.GET("/test", func(c *gin.Context) {
		gotRequestID = c.GetString("request_id")
		c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if gotRequestID == "" {
		t.Error("request_id should be generated")
	}
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Error("X-Request-ID response header should be set")
	}
	if got := w.Header().Get("X-Request-ID"); got != gotRequestID {
		t.Errorf("response header should match context value: %s != %s", got, gotRequestID)
	}
}

func TestRequestID_PreserveExisting(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())

	var gotRequestID string
	r.GET("/test", func(c *gin.Context) {
		gotRequestID = c.GetString("request_id")
		c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "existing-id-123")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if gotRequestID != "existing-id-123" {
		t.Errorf("expected existing-id-123, got %s", gotRequestID)
	}
	if got := w.Header().Get("X-Request-ID"); got != "existing-id-123" {
		t.Errorf("expected existing-id-123 in response, got %s", got)
	}
}
