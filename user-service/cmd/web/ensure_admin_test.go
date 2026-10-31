package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/handler"
	"github.com/stretchr/testify/require"
)

const testAdminPassword = "admin-password-123"

func login(t *testing.T, app http.Handler, userName, password string) int {
	t.Helper()

	rec := executeRequest(t, app, http.MethodPost, "/login-user", handler.LoginUserRequest{
		UserName: userName,
		Password: password,
	})
	return rec.Code
}

func TestEnsureAdminExists_CreatesAnAdminWhoCanLogIn(t *testing.T) {
	svc, db := newTestService(t)
	app := routes(handler.NewHandler(svc))

	require.NoError(t, svc.EnsureAdminExists("admin", "admin@example.com", testAdminPassword))

	var admin domain.User
	require.NoError(t, db.Where("user_name = ?", "admin").First(&admin).Error)
	require.Equal(t, domain.AdminRole, admin.Role)
	require.Equal(t, http.StatusOK, login(t, app, "admin", testAdminPassword))
}

func TestEnsureAdminExists_EmailIsStoredLowerCased(t *testing.T) {
	svc, db := newTestService(t)
	app := routes(handler.NewHandler(svc))

	require.NoError(t, svc.EnsureAdminExists("admin", "Admin@Example.COM", testAdminPassword))

	var admin domain.User
	require.NoError(t, db.Where("user_name = ?", "admin").First(&admin).Error)
	require.Equal(t, "admin@example.com", admin.Email)

	rec := executeRequest(t, app, http.MethodPost, "/register-customer", registerBody("bob", "admin@example.com"))
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestEnsureAdminExists_RejectsAnInvalidEmail(t *testing.T) {
	tooLongEmail := strings.Repeat("a", 64) + "@" +
		strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63)
	require.Greater(t, len(tooLongEmail), 254)

	tests := []struct {
		testName string
		email    string
	}{
		{testName: "empty", email: ""},
		{testName: "without @", email: "admin"},
		{testName: "without domain", email: "admin@"},
		{testName: "display-name form", email: "Admin <admin@example.com>"},
		{testName: "too long", email: tooLongEmail},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			svc, db := newTestService(t)

			require.Error(t, svc.EnsureAdminExists("admin", test.email, testAdminPassword))

			var count int64
			require.NoError(t, db.Model(&domain.User{}).Count(&count).Error)
			require.Zero(t, count, "no user may exist")
		})
	}
}

func TestEnsureAdminExists_ChangesNothingWhenTheUserNameExists(t *testing.T) {
	svc, db := newTestService(t)
	app := routes(handler.NewHandler(svc))

	require.NoError(t, svc.EnsureAdminExists("admin", "admin@example.com", testAdminPassword))
	require.NoError(t, svc.EnsureAdminExists("admin", "other@example.com", "another-password-456"))

	var admins []domain.User
	require.NoError(t, db.Find(&admins).Error)
	require.Len(t, admins, 1)
	require.Equal(t, "admin@example.com", admins[0].Email)
	require.Equal(t, http.StatusOK, login(t, app, "admin", testAdminPassword))
	require.Equal(t, http.StatusUnauthorized, login(t, app, "admin", "another-password-456"))
}
