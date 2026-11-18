package ai

import (
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBatchConversationRouteRegistersWithoutConflict(t *testing.T) {
	e := echo.New()
	NewHandler(nil).Register(e.Group("/api/v1"))
	for _, route := range e.Routes() {
		if route.Method == "POST" && route.Path == "/api/v1/ai/conversations/batch/delete" {
			return
		}
	}
	t.Fatalf("batch conversation delete route not registered; routes = %#v", e.Routes())
}
