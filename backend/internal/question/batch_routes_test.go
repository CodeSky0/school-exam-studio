package question

import (
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBatchQuestionRoutesRegisterWithoutConflict(t *testing.T) {
	e := echo.New()
	NewHandler(nil).Register(e.Group("/api/v1"))
	found := map[string]bool{}
	for _, route := range e.Routes() {
		found[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"PATCH /api/v1/admin/questions/batch/status",
		"POST /api/v1/questions/batch/delete",
	} {
		if !found[want] {
			t.Fatalf("route %s not registered; routes = %#v", want, found)
		}
	}
}
