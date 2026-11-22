package userclient_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/userclient"
	"github.com/stretchr/testify/require"
)

func TestGetUser_AsksForTheUserWithTheServiceToken(t *testing.T) {
	type seenRequest struct {
		method, path, auth string
	}
	seen := make(chan seenRequest, 1)

	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		seen <- seenRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization")}
		respondWith(http.StatusOK, aliceJSON)(w, r)
	})
	tokens := &fakeTokenSource{token: "service-token"}
	client := userclient.NewHTTPUserClient(srv.URL, tokens)

	_, err := client.GetUser(context.Background(), 7)
	require.NoError(t, err)

	got := <-seen
	require.Equal(t, http.MethodGet, got.method)
	require.Equal(t, "/internal/users/7", got.path)
	require.Equal(t, "Bearer service-token", got.auth)
	require.Equal(t, 1, tokens.calls, "the token source is asked once per call; caching is its job")
}

func TestGetUser_ReturnsTheUser(t *testing.T) {
	srv, _ := newServer(t, respondWith(http.StatusOK, aliceJSON))
	client := userclient.NewHTTPUserClient(srv.URL, &fakeTokenSource{token: "t"})

	user, err := client.GetUser(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, userclient.User{ID: 7, UserName: "alice", Email: "alice@example.com"}, user)
}

func TestGetUser_MapsUserServiceAnswers(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		expectedErr error // nil means an unknown error
	}{
		{name: "not found is a definite answer", status: http.StatusNotFound, expectedErr: userclient.ErrUserNotFound},
		{name: "bad request is not a statement about the user", status: http.StatusBadRequest},
		{name: "unauthorized is not a statement about the user", status: http.StatusUnauthorized},
		{name: "forbidden is not a statement about the user", status: http.StatusForbidden},
		{name: "server error is not a statement about the user", status: http.StatusInternalServerError},
		{name: "service unavailable is not a statement about the user", status: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv, _ := newServer(t, respondWith(test.status, ""))
			client := userclient.NewHTTPUserClient(srv.URL, &fakeTokenSource{token: "t"})

			_, err := client.GetUser(context.Background(), 7)

			require.Error(t, err)
			if test.expectedErr != nil {
				require.ErrorIs(t, err, test.expectedErr)
				return
			}

			require.False(t, errors.Is(err, userclient.ErrUserNotFound), "must not be mistaken for a definite answer")
		})
	}
}

func TestGetUser_WithoutAServiceTokenNothingIsSent(t *testing.T) {
	srv, requests := newServer(t, respondWith(http.StatusOK, aliceJSON))
	tokenErr := errors.New("user-service is down")
	client := userclient.NewHTTPUserClient(srv.URL, &fakeTokenSource{err: tokenErr})

	_, err := client.GetUser(context.Background(), 7)

	require.ErrorIs(t, err, tokenErr)
	require.False(t, errors.Is(err, userclient.ErrUserNotFound))
	require.Zero(t, requests.Load())
}

func TestGetUser_UserServiceIsUnreachable(t *testing.T) {
	srv, _ := newServer(t, respondWith(http.StatusOK, aliceJSON))
	url := srv.URL
	srv.Close() // nothing listens on this address any more
	client := userclient.NewHTTPUserClient(url, &fakeTokenSource{token: "t"})

	_, err := client.GetUser(context.Background(), 7)

	require.Error(t, err)
	require.False(t, errors.Is(err, userclient.ErrUserNotFound), "an outage says nothing about the user")
}

func TestGetUser_StopsWhenTheCallerGivesUp(t *testing.T) {
	// hang never answers; it returns when the client abandons the request.
	hang := func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }

	t.Run("context cancelled", func(t *testing.T) {
		srv, _ := newServer(t, hang)
		client := userclient.NewHTTPUserClient(srv.URL, &fakeTokenSource{token: "t"})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.GetUser(ctx, 7)

		require.ErrorIs(t, err, context.Canceled)
		require.False(t, errors.Is(err, userclient.ErrUserNotFound))
	})

	t.Run("deadline exceeded", func(t *testing.T) {
		srv, _ := newServer(t, hang)
		client := userclient.NewHTTPUserClient(srv.URL, &fakeTokenSource{token: "t"})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetUser(ctx, 7)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.False(t, errors.Is(err, userclient.ErrUserNotFound))
	})
}

func TestGetUser_AnAnswerItCannotReadIsAnError(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "not JSON", body: `<html>oops</html>`},
		{name: "cut off halfway", body: `{"item":{"id":7,"user_na`},
		{name: "empty", body: ``},
		{name: "user has no email", body: `{"item":{"id":7,"user_name":"alice"}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv, _ := newServer(t, respondWith(http.StatusOK, test.body))
			client := userclient.NewHTTPUserClient(srv.URL, &fakeTokenSource{token: "t"})

			_, err := client.GetUser(context.Background(), 7)

			require.Error(t, err)
			require.False(t, errors.Is(err, userclient.ErrUserNotFound))
		})
	}
}
