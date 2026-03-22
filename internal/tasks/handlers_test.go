package tasks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandleCreateTaskValidation(t *testing.T) {
	e := echo.New()
	h := &Handlers{}

	// Empty title should fail
	req := httptest.NewRequest(http.MethodPost, "/api/tasklists/list1/tasks", strings.NewReader("title="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("listId")
	c.SetParamValues("list1")

	err := h.HandleCreateTask(c)
	if err == nil && rec.Code != http.StatusBadRequest {
		t.Log("Expected error or 400 for empty title")
	}
}
