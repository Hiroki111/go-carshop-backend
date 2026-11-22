package userclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type fakeTokenSource struct {
	token string
	err   error
	calls int
}

func (f *fakeTokenSource) Token(ctx context.Context) (string, error) {
	f.calls++
	return f.token, f.err
}

func newServer(t *testing.T, handler http.HandlerFunc) (srv *httptest.Server, requests *atomic.Int32) {
	t.Helper()

	requests = &atomic.Int32{}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, requests
}

func respondWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

const aliceJSON = `{"item":{"id":7,"user_name":"alice","email":"alice@example.com"}}`
