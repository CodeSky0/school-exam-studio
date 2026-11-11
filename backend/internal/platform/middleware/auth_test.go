package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/ent/user"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
	"school-exam-studio/backend/internal/platform/platformtest"
)

const (
	middlewareSecret   = "01234567890123456789012345678901"
	middlewarePassword = "CorrectHorse123!"
)

func TestAuthenticationChecksCurrentUserOnEveryRequest(t *testing.T) {
	client := platformtest.Open(t)
	service := auth.NewService(
		auth.NewRepository(client),
		auth.NewPasswordHasher(bcrypt.MinCost),
		middlewareSecret,
		2*time.Hour,
	)
	ctx := context.Background()
	initialized, err := service.Initialize(ctx, auth.InitializeInput{
		SchoolName: "实验中学",
		AdminName:  "管理员",
		Email:      "admin@example.com",
		Password:   middlewarePassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, auth.LoginInput{
		SchoolCode: initialized.SchoolCode,
		Email:      "admin@example.com",
		Password:   middlewarePassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.GET("/protected", func(c echo.Context) error {
		principal, ok := platformmiddleware.PrincipalFrom(c)
		if !ok {
			t.Fatal("principal missing from request context")
		}
		return c.JSON(http.StatusOK, principal)
	}, platformmiddleware.Authenticate(service.Tokens(), service))
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: login.Token})
		response := httptest.NewRecorder()
		e.ServeHTTP(response, req)
		return response
	}
	if response := request(); response.Code != http.StatusOK {
		t.Fatalf("valid token returned %d: %s", response.Code, response.Body.String())
	}

	update := client.User.Update().Where(
		user.IDEQ(initialized.User.ID),
		user.SchoolIDEQ(initialized.SchoolID),
	).SetStatus(auth.StatusDisabled)
	if _, err := update.Save(ctx); err != nil {
		t.Fatal(err)
	}
	assertUnauthorized(t, request(), "disabled user")

	if _, err := client.User.Update().Where(
		user.IDEQ(initialized.User.ID),
		user.SchoolIDEQ(initialized.SchoolID),
	).SetStatus(auth.StatusActive).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if response := request(); response.Code != http.StatusOK {
		t.Fatalf("reactivated user returned %d", response.Code)
	}
	if _, err := client.User.Update().Where(
		user.IDEQ(initialized.User.ID),
		user.SchoolIDEQ(initialized.SchoolID),
	).SetRole(auth.RoleTeacher).Save(ctx); err != nil {
		t.Fatal(err)
	}
	assertUnauthorized(t, request(), "changed role")
	if _, err := client.User.Update().Where(
		user.IDEQ(initialized.User.ID),
		user.SchoolIDEQ(initialized.SchoolID),
	).SetRole(auth.RoleAdmin).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.User.Update().Where(
		user.IDEQ(initialized.User.ID),
		user.SchoolIDEQ(initialized.SchoolID),
	).AddSessionVersion(1).Save(ctx); err != nil {
		t.Fatal(err)
	}
	assertUnauthorized(t, request(), "changed session version")

	withoutCookie := httptest.NewRequest(http.MethodGet, "/protected", nil)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, withoutCookie)
	assertUnauthorized(t, response, "missing cookie")
}

func assertUnauthorized(t *testing.T, response *httptest.ResponseRecorder, scenario string) {
	t.Helper()
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("%s returned %d: %s", scenario, response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["request_id"] == "" || body["error"] == nil {
		t.Fatalf("%s returned invalid error envelope: %s", scenario, response.Body.String())
	}
}
