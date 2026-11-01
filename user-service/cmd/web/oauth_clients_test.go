package main

import (
	"strings"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/oauth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/service"
	"github.com/stretchr/testify/require"
)

var (
	orderServiceSecret   = strings.Repeat("o", 40)
	messageServiceSecret = strings.Repeat("m", 40)
)

func serviceWithConfiguredClients(t *testing.T) *service.Service {
	t.Helper()

	t.Setenv("ORDER_SERVICE_CLIENT_SECRET", orderServiceSecret)
	t.Setenv("MESSAGE_SERVICE_CLIENT_SECRET", messageServiceSecret)

	clients, err := newOAuthClientRegistry()
	require.NoError(t, err)

	return service.NewService(nil, testKey, clients)
}

func TestConfiguredClients_IssueServiceToken(t *testing.T) {
	tests := []struct {
		testName string
		clientID string
		secret   string
		scope    string

		expectedScopes []string
		expectedErr    error
	}{
		{
			testName: "message-service gets users:read when it asks for nothing in particular",
			clientID: "message-service", secret: messageServiceSecret,
			expectedScopes: []string{"users:read"},
		},
		{
			testName: "message-service asks for users:read",
			clientID: "message-service", secret: messageServiceSecret, scope: "users:read",
			expectedScopes: []string{"users:read"},
		},
		{testName: "fail - message-service asks for cars:reserve", clientID: "message-service", secret: messageServiceSecret, scope: "cars:reserve", expectedErr: oauth.ErrInvalidScope},
		{testName: "fail - message-service asks for cars:release", clientID: "message-service", secret: messageServiceSecret, scope: "cars:release", expectedErr: oauth.ErrInvalidScope},
		{testName: "fail - message-service asks for cars:sold", clientID: "message-service", secret: messageServiceSecret, scope: "cars:sold", expectedErr: oauth.ErrInvalidScope},
		{
			testName: "fail - message-service asks for users:read and a car scope",
			clientID: "message-service", secret: messageServiceSecret, scope: "users:read cars:reserve",
			expectedErr: oauth.ErrInvalidScope,
		},
		{
			testName: "fail - message-service uses order-service's secret",
			clientID: "message-service", secret: orderServiceSecret,
			expectedErr: oauth.ErrInvalidClient,
		},

		{testName: "fail - order-service asks for users:read", clientID: "order-service", secret: orderServiceSecret, scope: "users:read", expectedErr: oauth.ErrInvalidScope},
		{
			testName: "order-service still gets its car scopes, and not users:read",
			clientID: "order-service", secret: orderServiceSecret,
			expectedScopes: []string{"cars:reserve", "cars:release", "cars:sold"},
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			svc := serviceWithConfiguredClients(t)

			token, err := svc.IssueServiceToken(test.clientID, test.secret, test.scope)

			if test.expectedErr != nil {
				require.ErrorIs(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, test.expectedScopes, token.Scopes)
		})
	}
}

func TestNewOAuthClientRegistry_NeedsASecretForEveryClient(t *testing.T) {
	tests := []struct {
		testName      string
		orderSecret   string
		messageSecret string
	}{
		{testName: "message-service secret missing", orderSecret: orderServiceSecret, messageSecret: ""},
		{testName: "message-service secret too short", orderSecret: orderServiceSecret, messageSecret: "short"},
		{testName: "order-service secret missing", orderSecret: "", messageSecret: messageServiceSecret},
		{testName: "order-service secret too short", orderSecret: "short", messageSecret: messageServiceSecret},
		{testName: "both secrets missing", orderSecret: "", messageSecret: ""},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			t.Setenv("ORDER_SERVICE_CLIENT_SECRET", test.orderSecret)
			t.Setenv("MESSAGE_SERVICE_CLIENT_SECRET", test.messageSecret)

			_, err := newOAuthClientRegistry()

			require.Error(t, err)
		})
	}
}
