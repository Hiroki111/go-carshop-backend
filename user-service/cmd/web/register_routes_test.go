package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/handler"
	"github.com/stretchr/testify/require"
)

const validPassword = "i-like-oranges"

func registerBody(userName, email string) map[string]any {
	return map[string]any{"user_name": userName, "email": email, "password": validPassword}
}

// without returns a copy of the body with one field left out altogether.
func without(body map[string]any, field string) map[string]any {
	copied := make(map[string]any, len(body))
	for k, v := range body {
		if k != field {
			copied[k] = v
		}
	}
	return copied
}

func TestRegisterCustomer(t *testing.T) {
	const (
		existingUserName = "alice"
		existingEmail    = "alice@example.com"
	)

	tooLongEmail := strings.Repeat("a", 64) + "@" +
		strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63)
	require.Greater(t, len(tooLongEmail), 254)

	tests := []struct {
		testName      string
		body          any
		expectedCode  int
		expectedEmail string
	}{
		{
			testName:      "success",
			body:          registerBody("bob", "bob@example.com"),
			expectedCode:  http.StatusCreated,
			expectedEmail: "bob@example.com",
		},
		{
			testName:      "success - email is stored lower-cased",
			body:          registerBody("bob", "Bob@Example.COM"),
			expectedCode:  http.StatusCreated,
			expectedEmail: "bob@example.com",
		},

		{testName: "fail - email missing", body: without(registerBody("bob", ""), "email"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email empty", body: registerBody("bob", ""), expectedCode: http.StatusBadRequest},
		{testName: "fail - email without @", body: registerBody("bob", "bob.example.com"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email without local part", body: registerBody("bob", "@example.com"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email without domain", body: registerBody("bob", "bob@"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email with two @", body: registerBody("bob", "bob@bob@example.com"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email with a space inside", body: registerBody("bob", "bo b@example.com"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email in display-name form", body: registerBody("bob", "Bob <bob@example.com>"), expectedCode: http.StatusBadRequest},
		{testName: "fail - email with surrounding spaces", body: registerBody("bob", " bob@example.com "), expectedCode: http.StatusBadRequest},
		{testName: "fail - email too long", body: registerBody("bob", tooLongEmail), expectedCode: http.StatusBadRequest},
		{testName: "fail - user name missing", body: without(registerBody("bob", "bob@example.com"), "user_name"), expectedCode: http.StatusBadRequest},
		{testName: "fail - password missing", body: without(registerBody("bob", "bob@example.com"), "password"), expectedCode: http.StatusBadRequest},
		{
			testName:     "fail - password too short",
			body:         map[string]any{"user_name": "bob", "email": "bob@example.com", "password": "1234567"},
			expectedCode: http.StatusBadRequest,
		},
		{testName: "fail - malformed JSON", body: `{"user_name": `, expectedCode: http.StatusBadRequest},
		{testName: "fail - email is already registered", body: registerBody("bob", existingEmail), expectedCode: http.StatusConflict},
		{testName: "fail - email is already registered in a different case", body: registerBody("bob", "Alice@Example.COM"), expectedCode: http.StatusConflict},
		{testName: "fail - user name is taken", body: registerBody(existingUserName, "bob@example.com"), expectedCode: http.StatusConflict},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)
			existing := executeRequest(t, app, http.MethodPost, "/register-customer", registerBody(existingUserName, existingEmail))
			require.Equal(t, http.StatusCreated, existing.Code)

			rec := executeRequest(t, app, http.MethodPost, "/register-customer", test.body)
			require.Equal(t, test.expectedCode, rec.Code)

			var users []domain.User
			require.NoError(t, db.Order("id").Find(&users).Error)

			if test.expectedCode == http.StatusCreated {
				require.Len(t, users, 2)
				require.Equal(t, test.expectedEmail, users[1].Email)
				require.Equal(t, domain.CustomerRole, users[1].Role)
				return
			}

			require.Len(t, users, 1, "a rejected request must not create a user")
			require.Equal(t, existingUserName, users[0].UserName)
			require.Equal(t, existingEmail, users[0].Email)
		})
	}
}

// NOTE: Login will be done with email in the future.
func TestRegisterCustomer_ThenLoginWithUserName(t *testing.T) {
	app, _ := setupTestApp(t)

	registered := executeRequest(t, app, http.MethodPost, "/register-customer", registerBody("bob", "bob@example.com"))
	require.Equal(t, http.StatusCreated, registered.Code)

	login := executeRequest(t, app, http.MethodPost, "/login-user", handler.LoginUserRequest{
		UserName: "bob",
		Password: validPassword,
	})

	require.Equal(t, http.StatusOK, login.Code)
	require.NotEmpty(t, decodeBody[handler.LoginUserResponse](t, login).AccessToken)
}
